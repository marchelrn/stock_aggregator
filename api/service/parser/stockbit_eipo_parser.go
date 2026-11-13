package parser

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/marchelrn/stock_api/dto"
)

// StockbitEIPOParser parses Stockbit TC_EIPO (IPO Trade Confirmation) PDFs.
// Only PDFs where the Buy amount > 0 (i.e. user got an allotment) are processed.
// PDFs with Buy = 0 (refund-only, no allotment) are skipped via ErrEmailNotApplicable.
type StockbitEIPOParser struct{}

func NewStockbitEIPOParser() *StockbitEIPOParser {
	return &StockbitEIPOParser{}
}

func (p *StockbitEIPOParser) BrokerName() string { return "Stockbit EIPO" }

func (p *StockbitEIPOParser) Parse(data []byte) ([]dto.ParsedTrade, error) {
	content, err := extractPDFTextWithFallback(data)
	if err != nil || strings.TrimSpace(content) == "" {
		return nil, ErrEmailNotApplicable
	}
	return p.parseContent(content)
}

func (p *StockbitEIPOParser) ParseEmail(ctx context.Context, data []byte, meta EmailMetadata) ([]dto.ParsedTrade, error) {
	filename := strings.ToLower(meta.Filename)
	subj := strings.ToLower(meta.Subject)
	if !strings.Contains(filename, "eipo") && !strings.Contains(subj, "ipo") {
		return nil, ErrEmailNotApplicable
	}
	return p.Parse(data)
}

// parseContent extracts allotted IPO trades from the TC_EIPO PDF text.
// Format example:
//
//	REF #  Board  Share                           Lot  Quantity  Price  Buy     Refund
//	6374143  IPO  PRDL PT Prodia Diagnostic Line  1    100       120    12,000  0
func (p *StockbitEIPOParser) parseContent(content string) ([]dto.ParsedTrade, error) {
	// Must contain allotment date as a signal it's an EIPO PDF
	if !strings.Contains(content, "Allotment Date") {
		return nil, ErrEmailNotApplicable
	}

	// Extract allotment date
	tradeDate := extractEIPODate(content)

	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	trades := extractEIPOTrades(lines, tradeDate)
	if len(trades) == 0 {
		// No allotment (Buy = 0 for all rows, e.g. JELI refund-only)
		return nil, ErrEmailNotApplicable
	}
	return trades, nil
}

// datePattern matches formats like "07/07/2026" or "2026-07-07"
var eipoDatePatterns = []*regexp.Regexp{
	regexp.MustCompile(`Allotment\s+Date\s+(\d{2}/\d{2}/\d{4})`),
	regexp.MustCompile(`Allotment\s+Date\s+(\d{4}-\d{2}-\d{2})`),
}

func extractEIPODate(content string) time.Time {
	layouts := []string{"02/01/2006", "2006-01-02"}
	for i, re := range eipoDatePatterns {
		m := re.FindStringSubmatch(content)
		if len(m) >= 2 {
			t, err := time.Parse(layouts[i], m[1])
			if err == nil {
				return t
			}
		}
	}
	return time.Now()
}

// refNumPattern matches a REF # that is 5+ digits (possibly alphanumeric for older PDFs)
var eipoRefPattern = regexp.MustCompile(`^[A-Z0-9]{5,}$`)

// extractEIPOTrades finds rows in the IPO table where Buy > 0.
// The table header is "REF #  Board  Share  Lot  Quantity  Price  Buy  Refund".
// Cells may appear on separate lines (multi-line extraction) or one line.
func extractEIPOTrades(lines []string, tradeDate time.Time) []dto.ParsedTrade {
	boardPattern := regexp.MustCompile(`^(?:IPO|RG|NG|TN|UH)$`)

	// Try single-line format first:
	// e.g. "6374143 IPO PRDL PT Prodia Diagnostic Line Tbk 1 100 120 12,000 0"
	singleLine := regexp.MustCompile(`^([A-Z0-9]{5,})\s+(IPO|RG|NG|TN|UH)\s+([A-Z]{4})\s+.+?\s+(\d+)\s+(\d+)\s+([\d,]+)\s+([\d,]+)\s+([\d,]+)$`)

	var trades []dto.ParsedTrade

	// Find start of table (after "REF #" header)
	startIdx := -1
	endIdx := len(lines)
	for i, l := range lines {
		if strings.Contains(l, "REF #") || strings.Contains(l, "REF#") {
			startIdx = i
		}
		if startIdx >= 0 && (strings.Contains(l, "Your RDN") || strings.Contains(l, "Total Value") || strings.Contains(l, "Stamp Duty")) {
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

		// Single-line format
		if m := singleLine.FindStringSubmatch(line); m != nil {
			refNo := m[1]
			ticker := m[3]
			lot, _ := strconv.Atoi(m[4])
			price := parseEIPOFloat(m[6])
			buy := parseEIPOFloat(m[7])
			if lot > 0 && buy > 0 && price > 0 {
				gross := float64(lot) * 100 * price
				tax := gross * 0.0015 // assume standard buy levy; actual TC has stamp duty field
				trades = append(trades, dto.ParsedTrade{
					Ticker:     ticker,
					Type:       "BUY",
					Lot:        lot,
					Price:      price,
					Tax:        tax,
					AmountDone: gross + tax,
					Date:       tradeDate,
					RefNo:      refNo,
				})
			}
			i++
			continue
		}

		// Multi-line format: REF # on its own line
		if eipoRefPattern.MatchString(line) {
			var block []string
			for j := i; j < endIdx && j < i+15; j++ {
				if lines[j] != "" {
					block = append(block, lines[j])
				}
			}
			// block[0]=REF#, block[1]=Board, block[2]=ticker or start of share name
			if len(block) >= 7 && boardPattern.MatchString(safeGet(block, 1)) {
				refNo := block[0]
				// Find ticker (4 uppercase letters)
				ticker := ""
				tickerIdx := -1
				for bi, b := range block[2:] {
					if len(b) == 4 && isAllUpper(b) {
						ticker = b
						tickerIdx = bi + 2
						break
					}
				}
				if ticker == "" || tickerIdx < 0 {
					i++
					continue
				}
				// Numbers after ticker: Lot, Quantity, Price, Buy, Refund
				var nums []float64
				for _, b := range block[tickerIdx+1:] {
					if v := parseEIPOFloat(b); v > 0 || b == "0" {
						f, err := strconv.ParseFloat(strings.ReplaceAll(b, ",", ""), 64)
						if err == nil {
							nums = append(nums, f)
						}
					}
				}
				// Expect at least: lot, quantity, price, buy, refund
				if len(nums) >= 5 {
					lot := int(nums[0])
					price := nums[2]
					buy := nums[3]
					if lot > 0 && buy > 0 && price > 0 {
						gross := float64(lot) * 100 * price
						tax := gross * 0.0015
						trades = append(trades, dto.ParsedTrade{
							Ticker:     ticker,
							Type:       "BUY",
							Lot:        lot,
							Price:      price,
							Tax:        tax,
							AmountDone: gross + tax,
							Date:       tradeDate,
							RefNo:      refNo,
						})
					}
				}
				// Skip past the block
				consumed := 0
				for j := i; j < endIdx && consumed < len(block); j++ {
					if lines[j] != "" {
						consumed++
					}
					i = j + 1
				}
				continue
			}
		}

		i++
	}

	return trades
}

func safeGet(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}

func isAllUpper(s string) bool {
	for _, r := range s {
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func parseEIPOFloat(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// Ensure interface compliance (compile-time check)
var _ interface {
	BrokerName() string
	Parse([]byte) ([]dto.ParsedTrade, error)
	ParseEmail(context.Context, []byte, EmailMetadata) ([]dto.ParsedTrade, error)
} = (*StockbitEIPOParser)(nil)
