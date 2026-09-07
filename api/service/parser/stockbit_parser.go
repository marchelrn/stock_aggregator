package parser

import (
	"bytes"
	"fmt"
	"log"
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

// parseContent parses Stockbit trade confirmation format
// Format:
//
//	REF #  Board  Share                   Lot  Quantity  Price     Buy          Sell
//	0067289 RG    CDIA Chandra Daya...    6    600.00    705.00    423,000.00   0.00
func (p *StockbitParser) parseContent(content string) ([]dto.ParsedTrade, error) {
	var trades []dto.ParsedTrade

	// Extract transaction date
	tradeDate := p.extractTradeDate(content)

	lines := strings.Split(content, "\n")

	// Find the table section after "REF #" header
	inTradeTable := false

	// Pattern to match Stockbit trade rows:
	// REF#  Board  TICKER CompanyName  Lot  Quantity  Price  BuyAmount  SellAmount
	// e.g.: "0067289  RG  CDIA Chandra Daya Investasi Tbk.  6  600.00  705.00  423,000.00  0.00"
	tradeRowPattern := regexp.MustCompile(`^(\d{5,})\s+(RG|NG|TN|UH)\s+([A-Z]{4})\s+.+?\s+(\d+)\s+([\d,.]+)\s+([\d,.]+)\s+([\d,.]+)\s+([\d,.]+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "REF #") || strings.Contains(line, "REF#") {
			inTradeTable = true
			continue
		}

		// Stop at "Your RDN Details" or "Total Value"
		if strings.Contains(line, "Your RDN") || strings.Contains(line, "Total Value") {
			inTradeTable = false
			continue
		}

		if !inTradeTable {
			continue
		}

		matches := tradeRowPattern.FindStringSubmatch(line)
		if matches != nil {
			ticker := matches[3]
			lot := parseStockbitNumber(matches[4])
			price := parseStockbitFloat(matches[6])
			buyAmount := parseStockbitFloat(matches[7])
			sellAmount := parseStockbitFloat(matches[8])

			var tradeType string
			var amountDone float64
			if buyAmount > 0 {
				tradeType = "BUY"
				amountDone = buyAmount
			} else if sellAmount > 0 {
				tradeType = "SELL"
				amountDone = sellAmount
			} else {
				continue
			}

			trade := dto.ParsedTrade{
				Ticker:     ticker,
				Lot:        int(lot),
				Price:      price,
				Type:       tradeType,
				Date:       tradeDate,
				BrokerName: p.BrokerName(),
				AmountDone: amountDone,
			}

			trades = append(trades, trade)
			log.Printf("Stockbit parsed trade: %s %s %d lot @ %.2f = %.2f", tradeType, ticker, int(lot), price, amountDone)
			continue
		}

		// Alternative: try multiline parsing where data is on separate lines
		// Stockbit sometimes puts each field on its own line
		// Look for pattern: TICKER CompanyName\n on one line, then numbers on subsequent lines
	}

	// If table pattern didn't work, try line-by-line parsing for Stockbit's
	// multiline format where each field might be on a separate line
	if len(trades) == 0 {
		trades = p.parseMultilineFormat(lines, tradeDate)
	}

	return trades, nil
}

// parseMultilineFormat handles Stockbit PDFs where fields are on separate lines
// This happens when PDF text extraction puts each cell on its own line:
//
//	0067289
//	RG
//	CDIA Chandra Daya Investasi Tbk.
//	6
//	600.00
//	705.00
//	423,000.00
//	0.00
func (p *StockbitParser) parseMultilineFormat(lines []string, tradeDate time.Time) []dto.ParsedTrade {
	var trades []dto.ParsedTrade

	// Find ticker lines (4 uppercase letters followed by company name)
	tickerLinePattern := regexp.MustCompile(`^([A-Z]{4})\s+.+(?:Tbk\.|Ltd\.|Inc\.)`)
	refPattern := regexp.MustCompile(`^\d{5,}$`)

	for i, line := range lines {
		line = strings.TrimSpace(line)

		matches := tickerLinePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		ticker := matches[1]

		// Skip common non-ticker words
		if isCommonWord(ticker) {
			continue
		}

		// Verify there's a REF # before this line (within 3 lines)
		hasRef := false
		for j := max(0, i-3); j < i; j++ {
			if refPattern.MatchString(strings.TrimSpace(lines[j])) {
				hasRef = true
				break
			}
		}
		if !hasRef {
			continue
		}

		// Look for numbers after the ticker line
		var numbers []float64
		for j := i + 1; j < min(len(lines), i+8); j++ {
			nextLine := strings.TrimSpace(lines[j])
			if nextLine == "" {
				continue
			}
			// Stop if we hit another ticker or text section
			if tickerLinePattern.MatchString(nextLine) || refPattern.MatchString(nextLine) {
				break
			}
			n := parseStockbitFloat(nextLine)
			if n > 0 {
				numbers = append(numbers, n)
			}
		}

		// We expect: lot, quantity, price, buyAmount, sellAmount
		if len(numbers) < 4 {
			continue
		}

		lot := numbers[0]
		price := numbers[2]
		buyAmount := numbers[3]
		var sellAmount float64
		if len(numbers) >= 5 {
			sellAmount = numbers[4]
		}

		var tradeType string
		var amountDone float64
		if buyAmount > 0 && sellAmount == 0 {
			tradeType = "BUY"
			amountDone = buyAmount
		} else if sellAmount > 0 && buyAmount == 0 {
			tradeType = "SELL"
			amountDone = sellAmount
		} else if buyAmount > sellAmount {
			tradeType = "BUY"
			amountDone = buyAmount
		} else {
			tradeType = "SELL"
			amountDone = sellAmount
		}

		trade := dto.ParsedTrade{
			Ticker:     ticker,
			Lot:        int(lot),
			Price:      price,
			Type:       tradeType,
			Date:       tradeDate,
			BrokerName: p.BrokerName(),
			AmountDone: amountDone,
		}

		trades = append(trades, trade)
		log.Printf("Stockbit multiline parsed: %s %s %d lot @ %.2f = %.2f", tradeType, ticker, int(lot), price, amountDone)
	}

	return trades
}

func (p *StockbitParser) extractTradeDate(content string) time.Time {
	// Stockbit uses "Transaction Date\n21/05/2026" format
	txDatePattern := regexp.MustCompile(`Transaction Date\s*\n?\s*(\d{2}/\d{2}/\d{4})`)
	match := txDatePattern.FindStringSubmatch(content)
	if match != nil {
		t, err := time.Parse("02/01/2006", match[1])
		if err == nil {
			return t
		}
	}

	// Fallback: look for any date pattern
	datePattern := regexp.MustCompile(`(\d{2})/(\d{2})/(\d{4})`)
	match = datePattern.FindStringSubmatch(content)
	if match != nil {
		t, err := time.Parse("02/01/2006", match[0])
		if err == nil {
			return t
		}
	}

	return time.Now()
}

func parseStockbitNumber(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, ".", "")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

func parseStockbitFloat(s string) float64 {
	s = strings.TrimSpace(s)
	// Remove thousand separator commas: "423,000.00" -> "423000.00"
	// Detect format: if both , and . exist, and . comes last, it's international format
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		lastDot := strings.LastIndex(s, ".")
		lastComma := strings.LastIndex(s, ",")
		if lastDot > lastComma {
			// International: 423,000.00
			s = strings.ReplaceAll(s, ",", "")
		} else {
			// Indonesian: 423.000,00
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		}
	} else if strings.Contains(s, ",") {
		// Could be thousand sep or decimal
		parts := strings.Split(s, ",")
		if len(parts) == 2 && len(parts[1]) == 2 {
			// Decimal: "705,00"
			s = strings.Replace(s, ",", ".", 1)
		} else {
			// Thousand: "423,000"
			s = strings.ReplaceAll(s, ",", "")
		}
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
