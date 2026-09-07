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

// MandiriParser parses trade confirmation PDFs from Mandiri Sekuritas (MOST)
type MandiriParser struct{}

func NewMandiriParser() *MandiriParser {
	return &MandiriParser{}
}

func (p *MandiriParser) BrokerName() string {
	return "Mandiri Sekuritas"
}

// Parse reads a trade confirmation PDF from Mandiri Sekuritas and extracts trade data
func (p *MandiriParser) Parse(pdfBytes []byte) ([]dto.ParsedTrade, error) {
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
	log.Printf("PDF content length: %d chars", len(content))

	return p.parseContent(content)
}

// parseContent parses the text content extracted from a Mandiri Sekuritas trade confirmation PDF
// Mandiri Sekuritas trade confirmations typically contain:
// - Transaction date
// - Stock ticker/code
// - Buy/Sell indicator
// - Number of shares (lot * 100)
// - Price per share
// - Total amount
func (p *MandiriParser) parseContent(content string) ([]dto.ParsedTrade, error) {
	var trades []dto.ParsedTrade

	// Try to extract trade date from the document
	tradeDate := p.extractTradeDate(content)

	// Common patterns in Mandiri Sekuritas trade confirmations:
	// Pattern: TICKER | BUY/SELL | SHARES | PRICE | AMOUNT
	// The exact format may vary, so we try multiple patterns

	lines := strings.Split(content, "\n")

	// Pattern 1: Table row with ticker, type, shares, price
	// e.g., "BBCA  BUY  100  9,500  950,000"
	tickerPattern := regexp.MustCompile(`([A-Z]{4})\s+(BUY|SELL|BELI|JUAL|B|S)\s+([\d,\.]+)\s+([\d,\.]+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := tickerPattern.FindStringSubmatch(strings.ToUpper(line))
		if matches == nil {
			continue
		}

		ticker := matches[1]
		tradeType := normalizeTradeType(matches[2])
		shares := parseNumber(matches[3])
		price := parseNumber(matches[4])

		if shares <= 0 || price <= 0 {
			continue
		}

		lot := int(shares) / 100
		if lot <= 0 {
			lot = int(shares) // If shares < 100, treat as lot count
		}

		amountDone := shares * price

		trade := dto.ParsedTrade{
			Ticker:     ticker,
			Lot:        lot,
			Price:      price,
			Type:       tradeType,
			Date:       tradeDate,
			BrokerName: p.BrokerName(),
			AmountDone: amountDone,
		}

		trades = append(trades, trade)
		log.Printf("Parsed trade: %s %s %d lot @ %.2f", tradeType, ticker, lot, price)
	}

	if len(trades) == 0 {
		// Try alternative parsing pattern for different format
		trades = p.parseAlternativeFormat(lines, tradeDate)
	}

	return trades, nil
}

// parseAlternativeFormat tries a different parsing pattern
// Some Mandiri confirmations use a different table layout
func (p *MandiriParser) parseAlternativeFormat(lines []string, tradeDate time.Time) []dto.ParsedTrade {
	var trades []dto.ParsedTrade

	// Pattern: Look for 4-letter stock codes and nearby buy/sell + numbers
	stockCodePattern := regexp.MustCompile(`\b([A-Z]{4})\b`)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		matches := stockCodePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		ticker := matches[1]

		// Skip common non-ticker 4-letter words
		if isCommonWord(ticker) {
			continue
		}

		// Look in surrounding lines for trade details
		context := line
		if i+1 < len(lines) {
			context += " " + lines[i+1]
		}
		if i+2 < len(lines) {
			context += " " + lines[i+2]
		}

		contextUpper := strings.ToUpper(context)

		var tradeType string
		if strings.Contains(contextUpper, "BUY") || strings.Contains(contextUpper, "BELI") {
			tradeType = "BUY"
		} else if strings.Contains(contextUpper, "SELL") || strings.Contains(contextUpper, "JUAL") {
			tradeType = "SELL"
		} else {
			continue
		}

		// Extract numbers from context
		numbers := extractNumbers(context)
		if len(numbers) < 2 {
			continue
		}

		// Heuristic: first big number is likely shares/volume, second is price
		shares := numbers[0]
		price := numbers[1]

		if shares <= 0 || price <= 0 {
			continue
		}

		lot := int(shares) / 100
		if lot <= 0 {
			lot = int(shares)
		}

		trade := dto.ParsedTrade{
			Ticker:     ticker,
			Lot:        lot,
			Price:      price,
			Type:       tradeType,
			Date:       tradeDate,
			BrokerName: p.BrokerName(),
			AmountDone: shares * price,
		}

		trades = append(trades, trade)
	}

	return trades
}

// extractTradeDate tries to find the trade/transaction date from the document
func (p *MandiriParser) extractTradeDate(content string) time.Time {
	// Common date formats in Indonesian trade confirmations
	datePatterns := []struct {
		pattern *regexp.Regexp
		layout  string
	}{
		{regexp.MustCompile(`(\d{2})/(\d{2})/(\d{4})`), "02/01/2006"},
		{regexp.MustCompile(`(\d{2})-(\d{2})-(\d{4})`), "02-01-2006"},
		{regexp.MustCompile(`(\d{2})\s+(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+(\d{4})`), "02 Jan 2006"},
		{regexp.MustCompile(`(\d{2})\s+(Januari|Februari|Maret|April|Mei|Juni|Juli|Agustus|September|Oktober|November|Desember)\s+(\d{4})`), ""},
	}

	for _, dp := range datePatterns {
		match := dp.pattern.FindString(content)
		if match != "" {
			if dp.layout != "" {
				t, err := time.Parse(dp.layout, match)
				if err == nil {
					return t
				}
			} else {
				// Handle Indonesian month names
				t := parseIndonesianDate(match)
				if !t.IsZero() {
					return t
				}
			}
		}
	}

	return time.Now()
}

// Helper functions

func normalizeTradeType(t string) string {
	t = strings.ToUpper(strings.TrimSpace(t))
	switch t {
	case "BUY", "BELI", "B":
		return "BUY"
	case "SELL", "JUAL", "S":
		return "SELL"
	default:
		return t
	}
}

func parseNumber(s string) float64 {
	// Remove thousand separators (both . and ,)
	s = strings.TrimSpace(s)
	// Indonesian format: 1.000.000,50 or international: 1,000,000.50
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		// Determine which is decimal separator
		lastDot := strings.LastIndex(s, ".")
		lastComma := strings.LastIndex(s, ",")
		if lastComma > lastDot {
			// Indonesian format: dots are thousands, comma is decimal
			s = strings.ReplaceAll(s, ".", "")
			s = strings.ReplaceAll(s, ",", ".")
		} else {
			// International format: commas are thousands
			s = strings.ReplaceAll(s, ",", "")
		}
	} else if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", "")
	} else if strings.Contains(s, ".") {
		// Could be decimal or thousands separator
		// If only one dot and less than 3 digits after, treat as decimal
		parts := strings.Split(s, ".")
		if len(parts) == 2 && len(parts[1]) <= 2 {
			// Keep as decimal
		} else {
			s = strings.ReplaceAll(s, ".", "")
		}
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

func extractNumbers(s string) []float64 {
	re := regexp.MustCompile(`[\d,\.]+`)
	matches := re.FindAllString(s, -1)
	var numbers []float64
	for _, m := range matches {
		n := parseNumber(m)
		if n > 0 {
			numbers = append(numbers, n)
		}
	}
	return numbers
}

func isCommonWord(s string) bool {
	common := map[string]bool{
		"DARI": true, "YANG": true, "ATAU": true, "ATAS": true,
		"NAMA": true, "BANK": true, "DULU": true, "BARU": true,
		"NOTE": true, "DATE": true, "PAGE": true, "TOTAL": true,
		"FEES": true, "WITH": true, "THIS": true, "FROM": true,
		"MOST": true, "SELL": true, "BELI": true, "JUAL": true,
		"BUYS": true, "SALE": true, "CASH": true, "FUND": true,
		"COST": true, "RATE": true, "PAID": true, "FREE": true,
		"UNIT": true, "EACH": true, "TYPE": true, "CODE": true,
		"ACCT": true, "DESC": true, "OPEN": true, "HIGH": true,
		"LOWS": true, "LAST": true, "PREV": true, "NETT": true,
		"DONE": true, "TIME": true, "STMT": true, "INFO": true,
	}
	return common[s]
}

func parseIndonesianDate(s string) time.Time {
	months := map[string]string{
		"Januari": "January", "Februari": "February", "Maret": "March",
		"April": "April", "Mei": "May", "Juni": "June",
		"Juli": "July", "Agustus": "August", "September": "September",
		"Oktober": "October", "November": "November", "Desember": "December",
	}

	for id, en := range months {
		if strings.Contains(s, id) {
			s = strings.ReplaceAll(s, id, en)
			t, err := time.Parse("02 January 2006", s)
			if err == nil {
				return t
			}
		}
	}

	return time.Time{}
}
