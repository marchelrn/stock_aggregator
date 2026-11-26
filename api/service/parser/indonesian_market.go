package parser

import (
	"context"
	"regexp"
	"strings"
)

type TradeMarket uint8

const (
	TradeMarketUnknown TradeMarket = iota
	TradeMarketIndonesia
	TradeMarketExcluded
)

var (
	foreignInvestmentProduct = regexp.MustCompile(`(?i)\b(saham\s+(as|us|amerika(?:\s+serikat)?)|us\s+stocks?|ajaib\s+futures|(?:pasar|saham)\s+(?:luar\s+negeri|internasional)|nyse|nasdaq|lse|hkex|sgx|asx|forex|cfd)\b`)
	nonEquityTitle           = regexp.MustCompile(`(?im)^\s*(?:(?:rangkuman|konfirmasi|laporan)\s+transaksi\s+)?(?:obligasi|surat\s+utang|reksa\s*dana|aset\s+kripto|kripto|crypto)\b`)
	idMarketEvidence         = regexp.MustCompile(`(?i)\b(IDX|BEI|KSEI|KPEI|Bursa\s+Efek\s+Indonesia|RDN|Rekening\s+Dana\s+Nasabah)\b`)
	idCurrencyAmount         = regexp.MustCompile(`(?i)(?:\b(?:idr|rp\.?)\s*\d|\(\s*(?:idr|rp\.?)\s*\))`)
	foreignCurrencyAmount    = regexp.MustCompile(`(?i)(?:\b(?:usd|sgd|hkd|aud|eur|gbp|jpy)\s*\d|\(\s*(?:usd|sgd|hkd|aud|eur|gbp|jpy)\s*\)|\d\s+(?:usd|sgd|hkd|aud|eur|gbp|jpy)\b|\$\s*\d|\b(?:mata uang|currency)\s*:\s*(?:usd|sgd|hkd|aud|eur|gbp|jpy)\b)`)
	ajaibTableColumns        = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(ticker|efek|symbol|kode saham)\b`),
		regexp.MustCompile(`(?i)\b(harga|price)\b`),
		regexp.MustCompile(`(?i)\b(jumlah|lembar|shares|quantity|qty)\b`),
		regexp.MustCompile(`(?i)\b(bruto|gross|nilai|total\s+(beli|jual))\b`),
	}
	ajaibTicker     = regexp.MustCompile(`^[A-Z][A-Z0-9]{3}(?:-[A-Z0-9]+)?$`)
	ajaibNumber     = regexp.MustCompile(`^[0-9]+(?:[,.][0-9]+)*$`)
	ajaibSettlement = regexp.MustCompile(`(?i)\b(total\s+(bayar|terima|tagihan|keseluruhan)|net\s+(payable|proceeds))\s*(?:[:|]\s*)?(?:rp\.?\s*)?\(?[0-9]`)

	equityHeading        = regexp.MustCompile(`(?i)\b(trade\s+confirmation|contract\s+note|konfirmasi\s+transaksi|allotment\s+date)\b`)
	cashProductExclusion = regexp.MustCompile(`(?i)\b(kripto|crypto|reksa\s*dana|obligasi|surat\s+utang|bitcoin|usdt)\b`)
	nonTradeHeading      = regexp.MustCompile(`(?im)^\s*(statement\s+of\s+account|portfolio\s+statement|laporan\s+portofolio|laporan\s+bulanan|mutasi\s+rekening)\b`)
	nonDomesticCashBody  = regexp.MustCompile(`(?i)\b(?:produk|jenis\s+aset|kelas\s+aset|rekening|akun|tujuan)\s*:\s*(?:kripto|crypto|reksa\s*dana|obligasi|surat\s+utang|bitcoin|usdt|forex)\b|\b(?:deposit|top\s+up|withdrawal|penarikan)(?:\s+dana)?\s+(?:ke|dari|untuk)\s+(?:akun\s+)?(?:kripto|crypto|reksa\s*dana|obligasi|bitcoin|usdt|forex)\b`)
)

func classifyTradeMarket(broker, text string) TradeMarket {
	lines := strings.Split(text, "\n")
	var heading []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			heading = append(heading, line)
			if len(heading) == 12 {
				break
			}
		}
	}
	headerLines := strings.Join(heading, "\n")
	header := strings.Join(strings.Fields(headerLines), " ")
	document := strings.Join(strings.Fields(text), " ")
	if foreignInvestmentProduct.MatchString(header) || nonEquityTitle.MatchString(headerLines) || nonTradeHeading.MatchString(headerLines) || foreignCurrencyAmount.MatchString(document) {
		return TradeMarketExcluded
	}
	if !idMarketEvidence.MatchString(document) && !idCurrencyAmount.MatchString(document) {
		return TradeMarketUnknown
	}
	switch strings.ToLower(strings.TrimSpace(broker)) {
	case "ajaib sekuritas":
		if strings.Contains(strings.ToLower(header), "ajaib sekuritas") && ajaibSettlement.MatchString(document) && hasAjaibTradeTable(lines) {
			return TradeMarketIndonesia
		}
	case "stockbit sekuritas", "mandiri sekuritas", "trimegah sekuritas":
		lowerHeader := strings.ToLower(header)
		if equityHeading.MatchString(header) ||
			(strings.EqualFold(broker, "Stockbit Sekuritas") && strings.Contains(lowerHeader, "this is to confirm the following activity on your account")) ||
			(strings.EqualFold(broker, "Trimegah Sekuritas") && strings.Contains(lowerHeader, "trimegah")) {
			return TradeMarketIndonesia
		}
	}
	return TradeMarketUnknown
}

func hasAjaibTradeTable(lines []string) bool {
	headerFound := false
	for _, line := range lines {
		if headerFound {
			ticker, numbers := false, 0
			for _, field := range strings.Fields(strings.ReplaceAll(line, "|", " ")) {
				ticker = ticker || ajaibTicker.MatchString(field)
				if ajaibNumber.MatchString(field) {
					numbers++
				}
			}
			if ticker && numbers >= 3 {
				return true
			}
		}
		matched := 0
		for _, column := range ajaibTableColumns {
			if column.MatchString(line) {
				matched++
			}
		}
		if matched == len(ajaibTableColumns) {
			headerFound = true
		}
	}
	return false
}

func (d *BrokerDispatcherParser) ClassifyTradeMarket(ctx context.Context, data []byte, broker string) TradeMarket {
	if len(data) == 0 || len(data) > 8<<20 {
		return TradeMarketUnknown
	}
	passwords, _ := ctx.Value(pdfPasswordsContextKey{}).(map[string]string)
	text, err := extractLayoutPDF(ctx, data, passwords[strings.ToLower(strings.TrimSpace(broker))])
	if err != nil {
		return TradeMarketUnknown
	}
	return classifyTradeMarket(broker, text)
}

func domesticCashNotice(subject, body string) bool {
	if foreignInvestmentProduct.MatchString(subject) || cashProductExclusion.MatchString(subject) ||
		foreignInvestmentProduct.MatchString(body) || foreignCurrencyAmount.MatchString(body) || nonDomesticCashBody.MatchString(body) {
		return false
	}
	return idMarketEvidence.MatchString(body) || idCurrencyAmount.MatchString(body)
}
