package service

import (
	"bytes"
	"context"
	"errors"
	"net/mail"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/service/parser"
)

var soaFilename = regexp.MustCompile("(?i)^(?:[0-9]+[_-])?soa[0-9_-]+[.]pdf$")

const soaAmount = "((?:0|[1-9][0-9]{0,2}(?:,[0-9]{3})+|[1-9][0-9]*)(?:[.][0-9]{2})?)"

var exerciseRow = regexp.MustCompile(`(?i)^\s*(\d{2}/\d{2}/\d{4})\s+\d{2}/\d{2}/\d{4}\s+([RU])\s+(\d+)\s+Exercise\s+Right\s+([A-Z0-9]{4})-R\s+` + soaAmount + `\s+` + soaAmount + `(?:\s|$)`)
var exerciseLabel = regexp.MustCompile(`(?i)\bexercise\s+right\b`)

func parseRightExercises(layout string) ([]models.RightExercise, error) {
	upper := strings.ToUpper(layout)
	if !strings.Contains(upper, "STATEMENT OF ACCOUNT") {
		if strings.Contains(upper, "PORTFOLIO STATEMENT") && !exerciseLabel.MatchString(layout) {
			return nil, nil
		}
		return nil, errors.New("Format SOA Stockbit belum dikenali.")
	}
	if !strings.Contains(upper, "STOCKBIT SEKURITAS DIGITAL") {
		return nil, errors.New("Identitas broker pada SOA belum terbaca.")
	}
	type key struct {
		date           time.Time
		ticker, amount string
	}
	pairs := map[key]*models.RightExercise{}
	var order []key
	for _, line := range strings.Split(layout, "\n") {
		if !exerciseLabel.MatchString(line) {
			continue
		}
		m := exerciseRow.FindStringSubmatch(line)
		if m == nil {
			return nil, errors.New("Rincian penebusan rights belum terbaca.")
		}
		date, err := time.Parse("02/01/2006", m[1])
		if err != nil {
			return nil, errors.New("Tanggal penebusan rights tidak valid.")
		}
		debit, credit := strings.TrimSuffix(strings.ReplaceAll(m[5], ",", ""), ".00"), strings.TrimSuffix(strings.ReplaceAll(m[6], ",", ""), ".00")
		amount, other := credit, debit
		side := strings.ToUpper(m[2])
		if side == "U" {
			amount, other = debit, credit
		}
		if amount == "0" || other != "0" {
			return nil, errors.New("Nominal penebusan rights tidak sesuai.")
		}
		k := key{date, strings.ToUpper(m[4]), amount}
		pair := pairs[k]
		if pair == nil {
			pair = &models.RightExercise{Date: date, Ticker: k.ticker, Amount: amount}
			pairs[k] = pair
			order = append(order, k)
		}
		ref := &pair.CreditRef
		if side == "U" {
			ref = &pair.DebitRef
		}
		value := side + " " + m[3]
		if *ref != "" && *ref != value {
			return nil, errors.New("Pasangan referensi penebusan rights ambigu.")
		}
		*ref = value
	}
	var result []models.RightExercise
	for _, k := range order {
		p := pairs[k]
		if p.DebitRef == "" || p.CreditRef == "" {
			return nil, errors.New("Pasangan debit dan kredit penebusan rights belum lengkap.")
		}
		result = append(result, *p)
	}
	return result, nil
}

func (s *GmailServiceImpl) recordStockbitSOA(ctx context.Context, uid uint, sender, filename, messageID string, data []byte) error {
	broker := parser.BrokerScope(ctx)["stockbit sekuritas"]
	address, err := mail.ParseAddress(sender)
	if broker == 0 || s.Rights == nil || err != nil || !strings.HasSuffix(strings.ToLower(address.Address), "@stockbit.com") {
		return nil
	}
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-", "-")
	cmd.Stdin = bytes.NewReader(data)
	layout, err := cmd.Output()
	if err != nil {
		return errors.New("Teks SOA Stockbit belum terbaca.")
	}
	rows, err := parseRightExercises(string(layout))
	if err != nil {
		return err
	}
	for i := range rows {
		rows[i].UserID = uid
		rows[i].BrokerID = broker
		rows[i].Filename = filename
		rows[i].MessageID = messageID
	}
	return s.Rights.Record(ctx, rows)
}
