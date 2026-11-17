package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/models"
)

var (
	// Trimegah Deposit: "Top up RDN kamu sebesar Rp 649,141.00 berhasil dilakukan."
	reTrimegahDeposit = regexp.MustCompile(`(?i)(?:top\s+up\s+rdn|deposit).*?sebesar\s*(?:<[^>]+>\s*)*rp\s*([\d,.]+)`)

	// Trimegah Withdrawal: "Penarikan dana kamu sebesar Rp 1,000,000.00 telah berhasil diproses."
	reTrimegahWithdrawal = regexp.MustCompile(`(?i)(?:penarikan|tarik)\s+dana.*?sebesar\s*(?:<[^>]+>\s*)*rp\s*([\d,.]+)`)

	// Stockbit Deposit: "deposit yang kamu lakukan sebesar <strong>Rp 353,500</strong>"
	reStockbitDeposit    = regexp.MustCompile(`(?i)deposit\s+yang\s+kamu\s+lakukan\s+sebesar\s*(?:<[^>]+>\s*)*(?:idr|rp)?\s*([\d,.]+)`)
	reStockbitDepositAlt = regexp.MustCompile(`(?i)(?:deposit|dana\s+masuk).*?sebesar\s*(?:<[^>]+>\s*)*(?:idr|rp)?\s*([\d,.]+)`)

	// Stockbit Withdrawal:
	// "Jumlah : IDR 889,654.00"
	// "Nomor Referensi : 43877003"
	reStockbitWithdrawAmount    = regexp.MustCompile(`(?i)jumlah\s*(?:</td>\s*<td[^>]*>)?\s*:\s*(?:</td>\s*<td[^>]*>)?\s*(?:IDR|Rp)?\s*([\d,.]+)`)
	reStockbitWithdrawAmountAlt = regexp.MustCompile(`(?i)(?:penarikan|tarik\s+dana).*?sebesar\s*(?:<[^>]+>\s*)*(?:idr|rp)?\s*([\d,.]+)`)
	reStockbitWithdrawRef       = regexp.MustCompile(`(?i)nomor\s+referensi\s*(?:</td>\s*<td[^>]*>)?\s*:\s*(?:</td>\s*<td[^>]*>)?\s*([0-9a-zA-Z]+)`)

	// Bank Jago RDN Withdrawal (via Jago app → Stockbit RDN):
	// Subject: "Penarikan dana dari Kantong RDN berhasil"
	// Body patterns:
	//   "sebesar Rp2.297.040" / "sebesar Rp 2.297.040" / "jumlah Rp 889.654"
	reJagoWithdrawAmount = regexp.MustCompile(`(?i)(?:sebesar|jumlah|nominal|transfer)\s*(?:<[^>]+>\s*)*Rp\.?\s*([\d.,]+)`)

	// Dividen tunai (Stockbit: "Dividen Bersih\n5,546.51",
	// Mandiri: "Dividen Bersih (Rp)\t62,700"). Angka boleh berada di dalam
	// tag HTML, jadi tag (yang boleh berisi digit) dikonsumsi utuh lebih dulu.
	reDividendNet = regexp.MustCompile(`(?i)dividen\s+bersih(?:\s*\(?\s*rp\s*\)?)?(?:<[^>]*>|[^0-9<]){0,60}([0-9][\d,.]*)`)

	// Tanggal pembayaran dividen: ISO (Stockbit) atau "8 Mei 2026" (Mandiri).
	reDividendPayDateISO = regexp.MustCompile(`(?i)tanggal\s+pembayaran(?:<[^>]*>|[^0-9<]){0,60}(\d{4}-\d{2}-\d{2})`)
	reDividendPayDateID  = regexp.MustCompile(`(?i)pada\s+tanggal(?:<[^>]*>|[^0-9<]){0,60}(\d{1,2})\s+(januari|februari|maret|april|mei|juni|juli|agustus|september|oktober|november|desember)\s+(\d{4})`)

	// Ticker dari subjek: "(BBRI)" pada subjek Mandiri.
	reDividendTickerParen = regexp.MustCompile(`\(([A-Za-z]{4})\)`)
)

func cleanAndParseAmount(raw string) (float64, error) {
	// Format could be 649,141.00 or 353,500 or 1.000.000,00
	raw = strings.TrimSpace(raw)
	// Remove non-digit, non-comma, non-period
	cleaned := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == ',' || r == '.' {
			return r
		}
		return -1
	}, raw)

	// Determine if comma or period is the decimal separator
	lastComma := strings.LastIndex(cleaned, ",")
	lastDot := strings.LastIndex(cleaned, ".")

	if lastComma > lastDot && lastComma == len(cleaned)-3 {
		// Indonesian style: 1.000.000,00
		cleaned = strings.ReplaceAll(cleaned, ".", "")
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
	} else if lastDot > lastComma && lastDot == len(cleaned)-3 {
		// US style: 1,000,000.00
		cleaned = strings.ReplaceAll(cleaned, ",", "")
	} else {
		// No decimals or pure thousands separators
		cleaned = strings.ReplaceAll(cleaned, ",", "")
		cleaned = strings.ReplaceAll(cleaned, ".", "")
	}

	return strconv.ParseFloat(cleaned, 64)
}

// ParseCashEmail is retained only as a replay oracle for the old formats.
// Production Gmail sync uses CashEmailParser and never calls this function.
//
// Deprecated: use a validated MetadataParser.CashParser snapshot.
func ParseCashEmail(from, subject string, emailDate time.Time, body string) (*ParsedCashMutation, error) {
	fromLower := strings.ToLower(from)
	subjectLower := strings.ToLower(subject)

	// 1. Trimegah Sekuritas
	if strings.Contains(fromLower, "trimegah.com") || strings.Contains(fromLower, "trim.operation") {
		if strings.Contains(subjectLower, "top up") || strings.Contains(subjectLower, "deposit") {
			matches := reTrimegahDeposit.FindStringSubmatch(body)
			if len(matches) >= 2 {
				amount, err := cleanAndParseAmount(matches[1])
				if err != nil {
					return nil, fmt.Errorf("parse trimegah deposit amount: %w", err)
				}
				return &ParsedCashMutation{
					BrokerName:  "Trimegah Sekuritas",
					Type:        models.CashMutationDeposit,
					Amount:      amount,
					Date:        emailDate,
					Description: "Top Up RDN Trimegah Sekuritas",
				}, nil
			}
		}

		if strings.Contains(subjectLower, "tarik dana") || strings.Contains(subjectLower, "penarikan") || strings.Contains(subjectLower, "withdrawal") {
			matches := reTrimegahWithdrawal.FindStringSubmatch(body)
			if len(matches) >= 2 {
				amount, err := cleanAndParseAmount(matches[1])
				if err != nil {
					return nil, fmt.Errorf("parse trimegah withdrawal amount: %w", err)
				}
				return &ParsedCashMutation{
					BrokerName:  "Trimegah Sekuritas",
					Type:        models.CashMutationWithdraw,
					Amount:      amount,
					Date:        emailDate,
					Description: "Penarikan Dana Trimegah Sekuritas",
				}, nil
			}
		}
	}

	// 2. Stockbit Sekuritas
	if strings.Contains(fromLower, "stockbit.com") {
		if strings.Contains(subjectLower, "deposit") || strings.Contains(subjectLower, "dana masuk") {
			matches := reStockbitDeposit.FindStringSubmatch(body)
			if len(matches) < 2 {
				matches = reStockbitDepositAlt.FindStringSubmatch(body)
			}
			if len(matches) >= 2 {
				amount, err := cleanAndParseAmount(matches[1])
				if err != nil {
					return nil, fmt.Errorf("parse stockbit deposit amount: %w", err)
				}
				return &ParsedCashMutation{
					BrokerName:  "Stockbit Sekuritas",
					Type:        models.CashMutationDeposit,
					Amount:      amount,
					Date:        emailDate,
					Description: "Deposit Stockbit Sekuritas",
				}, nil
			}
		}

		if strings.Contains(subjectLower, "penarikan") || strings.Contains(subjectLower, "tarik dana") || strings.Contains(subjectLower, "withdrawal") {
			matches := reStockbitWithdrawAmount.FindStringSubmatch(body)
			if len(matches) < 2 {
				matches = reStockbitWithdrawAmountAlt.FindStringSubmatch(body)
			}
			if len(matches) >= 2 {
				amount, err := cleanAndParseAmount(matches[1])
				if err != nil {
					return nil, fmt.Errorf("parse stockbit withdrawal amount: %w", err)
				}

				ref := ""
				refMatches := reStockbitWithdrawRef.FindStringSubmatch(body)
				if len(refMatches) >= 2 {
					ref = strings.TrimSpace(refMatches[1])
				}

				return &ParsedCashMutation{
					BrokerName:  "Stockbit Sekuritas",
					Type:        models.CashMutationWithdraw,
					Amount:      amount,
					Date:        emailDate,
					RefNo:       ref,
					Description: "Penarikan Dana Stockbit Sekuritas",
				}, nil
			}
		}
	}

	// 3. Notifikasi dividen tunai (uang masuk ke RDN → kas bertambah)
	if parsed := parseDividendEmail(from, subject, emailDate, body); parsed != nil {
		return parsed, nil
	}

	return nil, nil
}

// parseDividendEmail menangani notifikasi dividen tunai dari broker:
//   - Stockbit : subjek "Pembagian dividen tunai BBRI", body "Dividen Bersih 5,546.51"
//   - Mandiri  : subjek "Pembayaran Dividen Tunai' ... (BBRI)'", body "Dividen Bersih (Rp) 62,700"
//
// Mengembalikan nil bila bukan email dividen broker atau gagal dibaca.
func parseDividendEmail(from, subject string, emailDate time.Time, body string) *ParsedCashMutation {
	fromLower := strings.ToLower(from)
	subjectLower := strings.ToLower(subject)

	isStockbit := strings.Contains(fromLower, "stockbit.com") &&
		strings.HasPrefix(subjectLower, "pembagian dividen tunai")
	isMandiri := strings.Contains(fromLower, "mandirisekuritas.co.id") &&
		strings.Contains(subjectLower, "pembayaran dividen tunai")
	if !isStockbit && !isMandiri {
		return nil
	}

	matches := reDividendNet.FindStringSubmatch(body)
	if len(matches) < 2 {
		return nil
	}
	amount, err := cleanAndParseAmount(matches[1])
	if err != nil || amount <= 0 {
		return nil
	}

	// Tanggal pembayaran; fallback ke tanggal email bila tidak ditemukan.
	payDate := emailDate
	if dm := reDividendPayDateISO.FindStringSubmatch(body); len(dm) >= 2 {
		if t, e := time.Parse("2006-01-02", dm[1]); e == nil {
			payDate = t
		}
	} else if dm := reDividendPayDateID.FindStringSubmatch(body); len(dm) >= 4 {
		if t, e := time.Parse("2 January 2006", fmt.Sprintf("%s %s %s", dm[1], monthID(dm[2]), dm[3])); e == nil {
			payDate = t
		}
	}

	brokerName := "Stockbit Sekuritas"
	if isMandiri {
		brokerName = "Mandiri Sekuritas"
	}

	ticker := dividendTicker(subject)
	description := "Dividen Tunai"
	if ticker != "" {
		description = "Dividen Tunai " + ticker
	}
	refNo := "DIV-" + ticker + "-" + payDate.Format("20060102")

	return &ParsedCashMutation{
		BrokerName:  brokerName,
		Type:        models.CashMutationDividend,
		Amount:      amount,
		Date:        payDate,
		RefNo:       refNo,
		Description: description,
	}
}

// dividendTicker mengambil kode saham dari subjek notifikasi dividen:
// "Pembagian dividen tunai BBRI" → BBRI, "… (BBRI)'" → BBRI.
func dividendTicker(subject string) string {
	if m := reDividendTickerParen.FindStringSubmatch(subject); len(m) >= 2 {
		return strings.ToUpper(m[1])
	}
	fields := strings.Fields(subject)
	if n := len(fields); n > 0 {
		last := strings.Trim(fields[n-1], "'\"")
		if len(last) >= 3 && len(last) <= 6 {
			if _, err := strconv.Atoi(last); err != nil {
				return strings.ToUpper(last)
			}
		}
	}
	return ""
}

// monthID menerjemahkan nama bulan Indonesia ke bahasa Inggris (time.Parse).
func monthID(month string) string {
	switch strings.ToLower(month) {
	case "januari":
		return "January"
	case "februari":
		return "February"
	case "maret":
		return "March"
	case "april":
		return "April"
	case "mei":
		return "May"
	case "juni":
		return "June"
	case "juli":
		return "July"
	case "agustus":
		return "August"
	case "september":
		return "September"
	case "oktober":
		return "October"
	case "november":
		return "November"
	case "desember":
		return "December"
	}
	return month
}
