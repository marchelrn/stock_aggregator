package parser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/marchelrn/stock_api/dto"
)

// TrimegahParser parses trade confirmation PDFs from PT Trimegah Sekuritas Indonesia Tbk
type TrimegahParser struct{}

func NewTrimegahParser() *TrimegahParser {
	return &TrimegahParser{}
}

func (p *TrimegahParser) BrokerName() string {
	return "Trimegah Sekuritas"
}

// Parse reads a trade confirmation PDF from Trimegah Sekuritas and extracts trade data
func (p *TrimegahParser) Parse(pdfBytes []byte) ([]dto.ParsedTrade, error) {
	return p.ParseWithMeta(pdfBytes, nil)
}

type visualTextItem struct {
	x, y float64
	s    string
}

func extractVisualTextFromPage(page pdf.Page) string {
	content := page.Content()
	if len(content.Text) == 0 {
		return ""
	}

	var items []visualTextItem
	var currentWord strings.Builder
	var lastX, lastY float64
	var wordStartX float64

	flush := func() {
		if currentWord.Len() > 0 {
			items = append(items, visualTextItem{x: wordStartX, y: lastY, s: currentWord.String()})
			currentWord.Reset()
		}
	}

	for _, t := range content.Text {
		if t.Y != lastY || t.X > lastX+15 {
			flush()
			wordStartX = t.X
		}
		currentWord.WriteString(t.S)
		lastX = t.X
		lastY = t.Y
	}
	flush()

	type lineGroup struct {
		y     float64
		items []visualTextItem
	}
	var lines []lineGroup
	for _, it := range items {
		matched := false
		for idx := range lines {
			if math.Abs(lines[idx].y-it.y) < 4.0 {
				lines[idx].items = append(lines[idx].items, it)
				matched = true
				break
			}
		}
		if !matched {
			lines = append(lines, lineGroup{y: it.y, items: []visualTextItem{it}})
		}
	}

	sort.Slice(lines, func(i, j int) bool {
		return lines[i].y > lines[j].y
	})

	var fullDoc strings.Builder
	for _, l := range lines {
		sort.Slice(l.items, func(a, b int) bool {
			return l.items[a].x < l.items[b].x
		})
		var lineStr strings.Builder
		for _, it := range l.items {
			lineStr.WriteString(fmt.Sprintf("%s ", it.s))
		}
		fullDoc.WriteString(strings.TrimSpace(lineStr.String()))
		fullDoc.WriteString("\n")
	}
	return fullDoc.String()
}

// ParseWithMeta reads a trade confirmation PDF with optional email metadata (for date/context fallback)
func (p *TrimegahParser) ParseWithMeta(pdfBytes []byte, meta *EmailMetadata) ([]dto.ParsedTrade, error) {
	// Fix non-standard PDF header if present (e.g. Trimegah generates "%PDF-1.3%\r\n")
	// ledongthuc/pdf requires buf[8] to be '\r' or '\n'
	if len(pdfBytes) > 10 && bytes.HasPrefix(pdfBytes, []byte("%PDF-1.")) && pdfBytes[8] != '\r' && pdfBytes[8] != '\n' {
		fixed := make([]byte, len(pdfBytes))
		copy(fixed, pdfBytes)
		fixed[8] = '\r'
		pdfBytes = fixed
	}

	reader := bytes.NewReader(pdfBytes)

	pdfReader, err := pdf.NewReader(reader, int64(len(pdfBytes)))
	if err != nil {
		// If data is already plain text (e.g. in test fixtures or extracted stream), parse directly
		if strings.Contains(strings.ToUpper(string(pdfBytes)), "TRIMEGAH") || strings.Contains(strings.ToUpper(string(pdfBytes)), "KONFIRMASI TRANSAKSI") {
			return p.parseContent(string(pdfBytes), meta)
		}
		return nil, fmt.Errorf("gagal membaca PDF Trimegah: %w", err)
	}

	var allText strings.Builder
	numPages := pdfReader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		// Try visual layout extraction first (groups words by Y/X coordinates)
		vText := extractVisualTextFromPage(page)
		if strings.TrimSpace(vText) != "" {
			allText.WriteString(vText)
			allText.WriteString("\n")
			continue
		}

		// Fallback to GetPlainText
		text, err := page.GetPlainText(nil)
		if err != nil {
			log.Printf("Warning: gagal membaca halaman %d PDF Trimegah: %v", i, err)
			continue
		}
		allText.WriteString(text)
		allText.WriteString("\n")
	}

	content := allText.String()
	log.Printf("Trimegah PDF content length: %d chars", len(content))
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("%w: PDF contains no extractable text", ErrEmailNotApplicable)
	}

	return p.parseContent(content, meta)
}

func (p *TrimegahParser) ParseEmail(ctx context.Context, data []byte, meta EmailMetadata) ([]dto.ParsedTrade, error) {
	trades, err := p.ParseWithMeta(data, &meta)
	if err != nil {
		if errors.Is(err, ErrEmailNotApplicable) {
			return nil, ErrEmailNotApplicable
		}
		return nil, err
	}
	return trades, nil
}

func (p *TrimegahParser) EmailSelectors(ctx context.Context) ([]EmailMetadata, error) {
	return []EmailMetadata{
		{From: "trim.operation@trimegah.com", Subject: "TC"},
		{From: "trim.operation@trimegah.com", Subject: "Trade Confirmation"},
	}, nil
}

type intermediateTrimegahTrade struct {
	RefNo       string
	Ticker      string
	CompanyName string
	Market      string
	Shares      float64
	Lot         int
	Price       float64
	Gross       float64
	Type        string
}

type trimegahFeeSummary struct {
	TotalPembebanan float64
	Komisi          float64
	Levy            float64
	PPN             float64
	KPEI            float64
	PajakPenjualan  float64
	EMeterai        float64
	WHT             float64
	NilaiBersih     float64
	TotalBeliGross  float64
	TotalJualGross  float64
}

// parseContent extracts trades, date, and fees from extracted Trimegah TC text
func (p *TrimegahParser) parseContent(content string, meta *EmailMetadata) ([]dto.ParsedTrade, error) {
	clientCode := p.extractClientCode(content, meta)
	tradeDate := p.extractTradeDate(content, meta)

	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	rawTrades := p.extractTrades(lines, clientCode, tradeDate)
	if len(rawTrades) == 0 {
		return nil, fmt.Errorf("tidak ada transaksi ditemukan pada PDF Trimegah")
	}

	feeSummary := p.extractSummaryFees(lines)

	var totalBuyGross, totalSellGross, totalGross float64
	for _, t := range rawTrades {
		gross := float64(t.Lot) * 100 * t.Price
		if gross <= 0 && t.Gross > 0 {
			gross = t.Gross
		}
		totalGross += gross
		if t.Type == "BUY" {
			totalBuyGross += gross
		} else {
			totalSellGross += gross
		}
	}

	totalFee := feeSummary.TotalPembebanan
	if totalFee == 0 {
		sum := feeSummary.Komisi + feeSummary.Levy + feeSummary.PPN + feeSummary.KPEI + feeSummary.PajakPenjualan + feeSummary.EMeterai + feeSummary.WHT
		if sum > 0 {
			totalFee = sum
		}
	}

	var trades []dto.ParsedTrade
	for _, t := range rawTrades {
		gross := float64(t.Lot) * 100 * t.Price
		if gross <= 0 && t.Gross > 0 {
			gross = t.Gross
		}

		var tax float64
		if totalFee > 0 && totalGross > 0 {
			tax = totalFee * (gross / totalGross)
		} else {
			// Fallback standard rate
			if t.Type == "BUY" {
				tax = gross * 0.0015
			} else {
				tax = gross * 0.0025
			}
		}

		tax = math.Round(tax*100) / 100
		var amountDone float64
		if t.Type == "BUY" {
			amountDone = math.Round((gross+tax)*100) / 100
		} else {
			amountDone = math.Round((gross-tax)*100) / 100
		}

		trade := dto.ParsedTrade{
			Ticker:     t.Ticker,
			Lot:        t.Lot,
			Price:      t.Price,
			Type:       t.Type,
			Date:       tradeDate,
			BrokerName: p.BrokerName(),
			AmountDone: amountDone,
			Tax:        tax,
			RefNo:      t.RefNo,
		}

		trades = append(trades, trade)
		log.Printf("Trimegah parsed trade: %s %s %d lot @ %.2f | Gross: %.2f | Tax: %.2f | AmountDone: %.2f",
			trade.Type, trade.Ticker, trade.Lot, trade.Price, gross, trade.Tax, trade.AmountDone)
	}

	return trades, nil
}

func (p *TrimegahParser) extractClientCode(content string, meta *EmailMetadata) string {
	// 1. Look for "Kode Nasabah : <Client Code>" in PDF content
	re := regexp.MustCompile(`(?i)Kode\s+Nasabah\s*[:\t ]+([A-Z0-9]+)`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	// 2. Look in email subject e.g. "TC <Date> - <Client Name> ( <Client Code> )"
	if meta != nil && meta.Subject != "" {
		subRe := regexp.MustCompile(`\(\s*([A-Z0-9]{3,})\s*\)`)
		if m := subRe.FindStringSubmatch(meta.Subject); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}

	// 3. Look in filename e.g. "TC_<ClientCode>-<Date>.pdf"
	if meta != nil && meta.Filename != "" {
		fnRe := regexp.MustCompile(`(?i)TC[_-]([A-Z0-9]+)`)
		if m := fnRe.FindStringSubmatch(meta.Filename); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}

	return ""
}

func (p *TrimegahParser) extractTrades(lines []string, clientCode string, tradeDate time.Time) []intermediateTrimegahTrade {
	var trades []intermediateTrimegahTrade

	// Pattern 1: Single line with all fields
	// JENIS TRX | JENIS PASAR | KODE EFEK | NAMA PERUSAHAAN | HARGA | JUMLAH (LEMBAR) | NILAI TRANSAKSI
	// e.g. "BELI RG MIDI MIDI UTAMA INDONESIA Tbk, PT 258 4,300 (1,109,400)"
	// or without company name: "BELI RG MIDI 258 4,300 (1,109,400)"
	singleLineFull := regexp.MustCompile(`(?i)^\s*(BELI|JUAL|BUY|SELL)\s+(RG|NG|TN|UH|IPO)\s+([A-Z0-9]{4}(?:-[A-Z0-9]+)?)\s+(.+?)\s+([\d,.]+)\s+([\d,.]+)\s+[\(\-]?([\d,.]+)[\)]?\s*$`)
	singleLineShort := regexp.MustCompile(`(?i)^\s*(BELI|JUAL|BUY|SELL)\s+(RG|NG|TN|UH|IPO)\s+([A-Z0-9]{4}(?:-[A-Z0-9]+)?)\s+([\d,.]+)\s+([\d,.]+)\s+[\(\-]?([\d,.]+)[\)]?\s*$`)

	typePattern := regexp.MustCompile(`^(?i)(BELI|JUAL|BUY|SELL)$`)
	marketPattern := regexp.MustCompile(`^(?i)(RG|NG|TN|UH|IPO)$`)
	tickerPattern := regexp.MustCompile(`^[A-Z0-9]{4}(?:-[A-Z0-9]+)?$`)

	endIdx := len(lines)
	for idx, l := range lines {
		lower := strings.ToLower(l)
		if strings.HasPrefix(lower, "total transaksi") ||
			strings.HasPrefix(lower, "total nilai") ||
			strings.HasPrefix(lower, "komisi") ||
			strings.HasPrefix(lower, "bank :") ||
			strings.HasPrefix(lower, "disclaimer") {
			endIdx = idx
			break
		}
	}

	i := 0
	for i < endIdx {
		line := lines[i]
		if line == "" {
			i++
			continue
		}

		// Try single line full
		if m := singleLineFull.FindStringSubmatch(line); len(m) == 8 {
			tradeType := normalizeTradeType(m[1])
			market := strings.ToUpper(m[2])
			ticker := strings.ToUpper(m[3])
			compName := strings.TrimSpace(m[4])
			price := parseTrimegahFloat(m[5])
			shares := parseTrimegahFloat(m[6])
			gross := parseTrimegahFloat(m[7])

			if shares > 0 && price > 0 {
				lot := int(math.Round(shares / 100.0))
				if lot <= 0 {
					lot = 1
				}
				refNo := generateTrimegahRef(clientCode, tradeDate, ticker, len(trades)+1)
				trades = append(trades, intermediateTrimegahTrade{
					RefNo:       refNo,
					Ticker:      ticker,
					CompanyName: compName,
					Market:      market,
					Shares:      shares,
					Lot:         lot,
					Price:       price,
					Gross:       gross,
					Type:        tradeType,
				})
				i++
				continue
			}
		}

		// Try single line short
		if m := singleLineShort.FindStringSubmatch(line); len(m) == 7 {
			tradeType := normalizeTradeType(m[1])
			market := strings.ToUpper(m[2])
			ticker := strings.ToUpper(m[3])
			price := parseTrimegahFloat(m[4])
			shares := parseTrimegahFloat(m[5])
			gross := parseTrimegahFloat(m[6])

			if shares > 0 && price > 0 {
				lot := int(math.Round(shares / 100.0))
				if lot <= 0 {
					lot = 1
				}
				refNo := generateTrimegahRef(clientCode, tradeDate, ticker, len(trades)+1)
				trades = append(trades, intermediateTrimegahTrade{
					RefNo:  refNo,
					Ticker: ticker,
					Market: market,
					Shares: shares,
					Lot:    lot,
					Price:  price,
					Gross:  gross,
					Type:   tradeType,
				})
				i++
				continue
			}
		}

		// Pattern 2: Multi-line cells
		// line matches BELI or JUAL
		if typePattern.MatchString(line) {
			var block []string
			var blockIndices []int
			for j := i; j < endIdx && len(block) < 10; j++ {
				if lines[j] != "" {
					block = append(block, lines[j])
					blockIndices = append(blockIndices, j)
				}
			}

			// block[0]: BELI / JUAL
			// block[1]: Market (RG)
			// block[2]: Ticker (MIDI)
			if len(block) >= 5 && marketPattern.MatchString(block[1]) && tickerPattern.MatchString(strings.ToUpper(block[2])) {
				tradeType := normalizeTradeType(block[0])
				market := strings.ToUpper(block[1])
				ticker := strings.ToUpper(block[2])

				var compName string
				var price, shares, gross float64
				lastIdx := i

				// Check if block[3] is company name or price
				if pVal, ok := tryParseTrimegahFloat(block[3]); ok {
					// block[3] is price, block[4] is shares, block[5] is gross
					price = pVal
					if len(block) > 4 {
						shares = parseTrimegahFloat(block[4])
					}
					if len(block) > 5 {
						gross = parseTrimegahFloat(block[5])
					}
					lastIdx = blockIndices[min(len(blockIndices)-1, 5)]
				} else {
					// block[3] is company name
					compName = block[3]
					if len(block) > 4 {
						price = parseTrimegahFloat(block[4])
					}
					if len(block) > 5 {
						shares = parseTrimegahFloat(block[5])
					}
					if len(block) > 6 {
						gross = parseTrimegahFloat(block[6])
					}
					lastIdx = blockIndices[min(len(blockIndices)-1, 6)]
				}

				if shares > 0 && price > 0 {
					lot := int(math.Round(shares / 100.0))
					if lot <= 0 {
						lot = 1
					}
					refNo := generateTrimegahRef(clientCode, tradeDate, ticker, len(trades)+1)
					trades = append(trades, intermediateTrimegahTrade{
						RefNo:       refNo,
						Ticker:      ticker,
						CompanyName: compName,
						Market:      market,
						Shares:      shares,
						Lot:         lot,
						Price:       price,
						Gross:       gross,
						Type:        tradeType,
					})
					i = lastIdx + 1
					continue
				}
			}
		}

		i++
	}

	return trades
}

func generateTrimegahRef(clientCode string, tradeDate time.Time, ticker string, seq int) string {
	dateStr := tradeDate.Format("20060102")
	if clientCode != "" {
		return fmt.Sprintf("TRIM-%s-%s-%s-%d", clientCode, dateStr, ticker, seq)
	}
	return fmt.Sprintf("TRIM-%s-%s-%d", dateStr, ticker, seq)
}

func (p *TrimegahParser) extractSummaryFees(lines []string) trimegahFeeSummary {
	var summary trimegahFeeSummary

	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}

		// Total Pembebanan : 2,014
		if strings.Contains(strings.ToLower(l), "total pembebanan") {
			summary.TotalPembebanan = extractTrailingAmount(l)
		} else if strings.Contains(strings.ToLower(l), "komisi perantara") || (strings.HasPrefix(strings.ToLower(l), "komisi") && !strings.Contains(strings.ToLower(l), "ppn")) {
			summary.Komisi = extractTrailingAmount(l)
		} else if strings.HasPrefix(strings.ToUpper(l), "LEVY") {
			summary.Levy = extractTrailingAmount(l)
		} else if strings.EqualFold(l, "PPN") || strings.HasPrefix(strings.ToUpper(l), "PPN ") || strings.HasPrefix(strings.ToUpper(l), "PPN:") {
			summary.PPN = extractTrailingAmount(l)
		} else if strings.HasPrefix(strings.ToUpper(l), "KPEI") {
			summary.KPEI = extractTrailingAmount(l)
		} else if strings.Contains(strings.ToLower(l), "pajak penjualan") {
			summary.PajakPenjualan = extractTrailingAmount(l)
		} else if strings.Contains(strings.ToLower(l), "e-meterai") || strings.Contains(strings.ToLower(l), "meterai") {
			summary.EMeterai = extractTrailingAmount(l)
		} else if strings.HasPrefix(strings.ToUpper(l), "WHT") {
			summary.WHT = extractTrailingAmount(l)
		} else if strings.Contains(strings.ToLower(l), "nilai transaksi bersih") {
			summary.NilaiBersih = extractTrailingAmount(l)
		} else if strings.Contains(strings.ToLower(l), "total transaksi beli") {
			summary.TotalBeliGross = extractTrailingAmount(l)
		} else if strings.Contains(strings.ToLower(l), "total transaksi jual") {
			summary.TotalJualGross = extractTrailingAmount(l)
		}
	}

	return summary
}

func extractTrailingAmount(line string) float64 {
	// Look for the last number token in the line (handles "(1,109,400)", "2,014", "1,382", etc.)
	re := regexp.MustCompile(`[\(\-]?[\d,]+(?:\.\d+)?[\)]?`)
	matches := re.FindAllString(line, -1)
	if len(matches) == 0 {
		return 0
	}
	last := matches[len(matches)-1]
	return parseTrimegahFloat(last)
}

func (p *TrimegahParser) extractTradeDate(content string, meta *EmailMetadata) time.Time {
	// 1. In email subject e.g. "TC <Day> <Month> <Year> - <Client Name>"
	if meta != nil && meta.Subject != "" {
		subjRe := regexp.MustCompile(`(?i)\bTC\s+([0-9]{1,2})\s+([A-Za-z]+)\s+([0-9]{4})\b`)
		if m := subjRe.FindStringSubmatch(meta.Subject); len(m) == 4 {
			if t, ok := parseTrimegahDateParts(m[1], m[2], m[3]); ok {
				return t
			}
		}
	}

	// 2. In PDF content lines e.g. "Tanggal : 30 Oktober 2026"
	lines := strings.Split(content, "\n")
	dateLineRe := regexp.MustCompile(`(?i)Tanggal\s*[:\t ]+([0-9]{1,2})\s+([A-Za-z]+)\s+([0-9]{4})`)
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(strings.ToLower(l), "pembayaran") {
			continue // Avoid Tanggal Pembayaran
		}
		if m := dateLineRe.FindStringSubmatch(l); len(m) == 4 {
			if t, ok := parseTrimegahDateParts(m[1], m[2], m[3]); ok {
				return t
			}
		}
	}

	// 3. Slash or dash date e.g. "Tanggal : 30/10/2026" or "30-10-2026"
	slashRe := regexp.MustCompile(`(?i)Tanggal\s*[:\t ]+([0-9]{1,2})[/-]([0-9]{1,2})[/-]([0-9]{4})`)
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(strings.ToLower(l), "pembayaran") {
			continue
		}
		if m := slashRe.FindStringSubmatch(l); len(m) == 4 {
			day, _ := strconv.Atoi(m[1])
			month, _ := strconv.Atoi(m[2])
			year, _ := strconv.Atoi(m[3])
			if day > 0 && day <= 31 && month > 0 && month <= 12 && year >= 2000 {
				return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			}
		}
	}

	return time.Now()
}

func parseTrimegahDateParts(dayStr, monthStr, yearStr string) (time.Time, bool) {
	day, err1 := strconv.Atoi(dayStr)
	year, err2 := strconv.Atoi(yearStr)
	if err1 != nil || err2 != nil {
		return time.Time{}, false
	}

	monthMap := map[string]time.Month{
		"januari":   time.January,
		"jan":       time.January,
		"january":   time.January,
		"februari":  time.February,
		"feb":       time.February,
		"february":  time.February,
		"maret":     time.March,
		"mar":       time.March,
		"march":     time.March,
		"april":     time.April,
		"apr":       time.April,
		"mei":       time.May,
		"may":       time.May,
		"juni":      time.June,
		"jun":       time.June,
		"june":      time.June,
		"juli":      time.July,
		"jul":       time.July,
		"july":      time.July,
		"agustus":   time.August,
		"agu":       time.August,
		"ags":       time.August,
		"aug":       time.August,
		"august":    time.August,
		"september": time.September,
		"sep":       time.September,
		"oktober":   time.October,
		"okt":       time.October,
		"oct":       time.October,
		"october":   time.October,
		"november":  time.November,
		"nov":       time.November,
		"desember":  time.December,
		"des":       time.December,
		"dec":       time.December,
		"december":  time.December,
	}

	month, ok := monthMap[strings.ToLower(monthStr)]
	if !ok {
		return time.Time{}, false
	}

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), true
}

func tryParseTrimegahFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")
	s = strings.TrimSpace(s)

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
		if len(parts) == 2 && len(parts[1]) <= 2 {
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

func parseTrimegahFloat(s string) float64 {
	val, _ := tryParseTrimegahFloat(s)
	return val
}
