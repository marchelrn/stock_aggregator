package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/models"
)

type ParsedCashMutation struct {
	BrokerName  string                  `json:"broker_name"`
	Type        models.CashMutationType `json:"type"` // DEPOSIT or WITHDRAW
	Amount      float64                 `json:"amount"`
	Date        time.Time               `json:"date"`
	RefNo       string                  `json:"ref_no,omitempty"`
	Description string                  `json:"description,omitempty"`
}

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

// ParseCashEmail parses a cash mutation email from its sender, subject, date, and body HTML/text.
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

	return nil, nil
}
