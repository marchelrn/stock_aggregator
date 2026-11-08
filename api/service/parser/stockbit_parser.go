package parser

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/marchelrn/stock_api/dto"
)

// StockbitParser parses trade confirmation PDFs from Stockbit Sekuritas
type StockbitParser struct{}

func NewStockbitParser() *StockbitParser {
	return &StockbitParser{}
}

func (p *StockbitParser) BrokerName() string {
	return "Stockbit Sekuritas"
}

// Parse reads a trade confirmation PDF from Stockbit and extracts trade data
func (p *StockbitParser) Parse(pdfBytes []byte) ([]dto.ParsedTrade, error) {
	reader := bytes.NewReader(pdfBytes)

	pdfReader, err := pdf.NewReader(reader, int64(len(pdfBytes)))
	if err != nil {
		return nil, fmt.Errorf("gagal membaca PDF: %w", err)
	}

	var allText strings.Builder
	numPages := pdfReader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			log.Printf("Warning: gagal membaca halaman %d: %v", i, err)
			continue
		}
		allText.WriteString(text)
		allText.WriteString("\n")
	}

	content := allText.String()
	log.Printf("Stockbit PDF content length: %d chars", len(content))

	return p.parseContent(content)
}

func (p *StockbitParser) ParseEmail(ctx context.Context, data []byte, meta EmailMetadata) ([]dto.ParsedTrade, error) {
	return p.Parse(data)
}

func (p *StockbitParser) EmailSelectors(ctx context.Context) ([]EmailMetadata, error) {
	return []EmailMetadata{
		{From: "no-reply@stockbit.com", Subject: "Trade Confirmation"},
	}, nil
}

type intermediateStockbitTrade struct {
	RefNo  string
	Ticker string
	Lot    int
	Price  float64
	Type   string
}

type stockbitFeeSummary struct {
	Commission float64
	VATComm    float64
	IDXFee     float64
	VATLevy    float64
	IncomeTax  float64
	StampDuty  float64
	TotalFee   float64
	TotalCost  float64
	TotalValue float64
}

// parseContent extracts trade details and fee/tax information from Stockbit trade confirmations
func (p *StockbitParser) parseContent(content string) ([]dto.ParsedTrade, error) {
	tradeDate := p.extractTradeDate(content)

	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	rawTrades := p.extractTrades(lines)
	if len(rawTrades) == 0 {
		return nil, fmt.Errorf("tidak ada transaksi ditemukan pada PDF Stockbit")
	}

	buyFees, sellFees := p.extractSummaryFees(lines)

	var totalBuyValue, totalSellValue float64
	for _, t := range rawTrades {
		gross := float64(t.Lot) * 100 * t.Price
		if t.Type == "BUY" {
			totalBuyValue += gross
		} else {
			totalSellValue += gross
		}
	}

	var trades []dto.ParsedTrade
	for _, t := range rawTrades {
		gross := float64(t.Lot) * 100 * t.Price
		var tax float64
		if t.Type == "BUY" {
			if totalBuyValue > 0 && buyFees.TotalFee > 0 {
				tax = buyFees.TotalFee * (gross / totalBuyValue)
			} else {
				tax = gross * 0.0015
			}
		} else {
			if totalSellValue > 0 && sellFees.TotalFee > 0 {
				tax = sellFees.TotalFee * (gross / totalSellValue)
			} else {
				tax = gross * 0.0025
			}
		}

		tax = math.Round(tax*100) / 100
		amountDone := math.Round((gross+tax)*100) / 100

		trade := dto.ParsedTrade{
			Ticker:     t.Ticker,
			Lot:        t.Lot,
			Price:      t.Price,
			Type:       t.Type,
			Date:       tradeDate,
			BrokerName: p.BrokerName(),
			AmountDone: amountDone,
			Tax:        tax,
		}

		trades = append(trades, trade)
		log.Printf("Stockbit parsed trade: %s %s %d lot @ %.2f | Gross: %.2f | Tax: %.2f | AmountDone: %.2f",
			trade.Type, trade.Ticker, trade.Lot, trade.Price, gross, trade.Tax, trade.AmountDone)
	}

	return trades, nil
}

func (p *StockbitParser) extractTrades(lines []string) []intermediateStockbitTrade {
	var trades []intermediateStockbitTrade

	refPattern := regexp.MustCompile(`^\d{5,}$`)
	boardPattern := regexp.MustCompile(`^(?:RG|NG|TN|UH)$`)
	tickerLinePattern := regexp.MustCompile(`^([A-Z]{4})\s+`)
	tradeRowSingleLine := regexp.MustCompile(`^(\d{5,})\s+(RG|NG|TN|UH)\s+([A-Z]{4})\s+.+?\s+([\d,.]+)\s+([\d,.]+)\s+([\d,.]+)\s+([\d,.]+)\s+([\d,.]+)$`)

	startIdx := -1
	endIdx := len(lines)
	for i, line := range lines {
		if strings.Contains(line, "REF #") || strings.Contains(line, "REF#") {
			startIdx = i
		}
		if startIdx >= 0 && (strings.Contains(line, "Your RDN") || strings.Contains(line, "Total Value")) {
			endIdx = i
			break
		}
	}
	if startIdx < 0 {
		startIdx = 0
	}

	i := startIdx
	for i < endIdx {
		line := lines[i]

		// Format 1: Multi-line cells (PDF text extraction per cell)
		if refPattern.MatchString(line) {
			var block []string
			for j := i; j < endIdx && j < i+12; j++ {
				if lines[j] != "" {
					block = append(block, lines[j])
				}
			}

			// Expected order:
			// 0: REF #
			// 1: Board
			// 2: Share (TICKER CompanyName)
			// 3: Lot
			// 4: Quantity
			// 5: Price
			// 6: Buy
			// 7: Sell
			if len(block) >= 8 && refPattern.MatchString(block[0]) && boardPattern.MatchString(block[1]) {
				tMatch := tickerLinePattern.FindStringSubmatch(block[2])
				if tMatch != nil {
					ticker := tMatch[1]
					lot := parseStockbitNumber(block[3])
					price := parseStockbitFloat(block[5])
					buyVal := parseStockbitFloat(block[6])
					sellVal := parseStockbitFloat(block[7])

					tradeType := ""
					if buyVal > 0 && sellVal == 0 {
						tradeType = "BUY"
					} else if sellVal > 0 && buyVal == 0 {
						tradeType = "SELL"
					} else if buyVal > sellVal {
						tradeType = "BUY"
					} else if sellVal > 0 {
						tradeType = "SELL"
					}

					if tradeType != "" && lot > 0 && price > 0 {
						trades = append(trades, intermediateStockbitTrade{
							RefNo:  block[0],
							Ticker: ticker,
							Lot:    int(lot),
							Price:  price,
							Type:   tradeType,
						})
						i += 8
						continue
					}
				}
			}
		}

		// Format 2: Single-line row
		if m := tradeRowSingleLine.FindStringSubmatch(line); m != nil {
			ticker := m[3]
			lot := parseStockbitNumber(m[4])
			price := parseStockbitFloat(m[6])
			buyVal := parseStockbitFloat(m[7])
			sellVal := parseStockbitFloat(m[8])

			tradeType := ""
			if buyVal > 0 && sellVal == 0 {
				tradeType = "BUY"
			} else if sellVal > 0 {
				tradeType = "SELL"
			}

			if tradeType != "" && lot > 0 && price > 0 {
				trades = append(trades, intermediateStockbitTrade{
					RefNo:  m[1],
					Ticker: ticker,
					Lot:    int(lot),
					Price:  price,
					Type:   tradeType,
				})
			}
		}

		i++
	}

	return trades
}

func (p *StockbitParser) extractSummaryFees(lines []string) (buyFees stockbitFeeSummary, sellFees stockbitFeeSummary) {
	getNums := func(label string) (float64, float64) {
		for i, line := range lines {
			if strings.EqualFold(line, label) || strings.HasPrefix(strings.ToLower(line), strings.ToLower(label)) {
				var nums []float64
				for j := i + 1; j < len(lines) && j < i+6; j++ {
					l := lines[j]
					if l == "" {
						continue
					}
					if isStockbitLabel(l) {
						break
					}
					val, ok := tryParseStockbitFloat(l)
					if ok {
						nums = append(nums, val)
						if len(nums) == 2 {
							break
						}
					}
				}
				if len(nums) == 2 {
					return nums[0], nums[1]
				} else if len(nums) == 1 {
					return nums[0], 0
				}
			}
		}
		return 0, 0
	}

	buyFees.Commission, sellFees.Commission = getNums("Commission")
	buyFees.VATComm, sellFees.VATComm = getNums("V.A.T Commission")
	buyFees.IDXFee, sellFees.IDXFee = getNums("IDX Fee")
	buyFees.VATLevy, sellFees.VATLevy = getNums("V.A.T Levy")
	buyFees.IncomeTax, sellFees.IncomeTax = getNums("Income Tax")
	buyFees.StampDuty, sellFees.StampDuty = getNums("Stamp Duty")
	buyFees.TotalCost, sellFees.TotalCost = getNums("Total Cost")
	buyFees.TotalValue, sellFees.TotalValue = getNums("Total Value")

	buyFees.TotalFee = buyFees.Commission + buyFees.VATComm + buyFees.IDXFee + buyFees.VATLevy + buyFees.IncomeTax + buyFees.StampDuty
	if buyFees.TotalFee == 0 && buyFees.TotalCost > buyFees.TotalValue && buyFees.TotalValue > 0 {
		buyFees.TotalFee = buyFees.TotalCost - buyFees.TotalValue
	}

	sellFees.TotalFee = sellFees.Commission + sellFees.VATComm + sellFees.IDXFee + sellFees.VATLevy + sellFees.IncomeTax + sellFees.StampDuty
	if sellFees.TotalFee == 0 && sellFees.TotalValue > sellFees.TotalCost && sellFees.TotalCost > 0 {
		sellFees.TotalFee = sellFees.TotalValue - sellFees.TotalCost
	}

	return buyFees, sellFees
}

func isStockbitLabel(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	labels := []string{
		"total value", "total fee", "commission", "v.a.t commission", "idx fee",
		"v.a.t levy", "income tax", "stamp duty", "total cost", "payment due",
		"for further", "ref #", "board", "share",
	}
	for _, l := range labels {
		if strings.HasPrefix(s, l) {
			return true
		}
	}
	return false
}

func (p *StockbitParser) extractTradeDate(content string) time.Time {
	txDatePattern := regexp.MustCompile(`(?i)Transaction Date\s*[:\r\n\t ]+([0-9]{2}/[0-9]{2}/[0-9]{4})`)
	match := txDatePattern.FindStringSubmatch(content)
	if match != nil {
		t, err := time.Parse("02/01/2006", match[1])
		if err == nil {
			return t
		}
	}

	datePattern := regexp.MustCompile(`\b([0-9]{2})/([0-9]{2})/([0-9]{4})\b`)
	match = datePattern.FindStringSubmatch(content)
	if match != nil {
		t, err := time.Parse("02/01/2006", match[0])
		if err == nil {
			return t
		}
	}

	return time.Now()
}

func tryParseStockbitFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		lastDot := strings.LastIndex(s, ".")
		lastComma := strings.LastIndex(s, ",")
		if lastDot > lastComma {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		}
	} else if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		if len(parts) == 2 && len(parts[1]) == 2 {
			s = strings.Replace(s, ",", ".", 1)
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

func parseStockbitFloat(s string) float64 {
	val, _ := tryParseStockbitFloat(s)
	return val
}

func parseStockbitNumber(s string) float64 {
	return parseStockbitFloat(s)
}
