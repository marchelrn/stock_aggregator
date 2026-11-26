package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/dto"
)

var cellGap = regexp.MustCompile(`\s{2,}|\s*\|\s*`)
var ajaibDate = regexp.MustCompile(`(?i)^(?:(?:tgl\.|tanggal) transaksi\s*:\s*)?(\d{1,2} [A-Za-z]+ \d{4}|\d{4}-\d{2}-\d{2})$`)
var columnMeanings = map[string]*regexp.Regexp{
	"ticker": regexp.MustCompile(`(?i)^(ticker|efek|kode saham|symbol)$`),
	"shares": regexp.MustCompile(`(?i)^(jumlah( lembar)?|lembar|quantity|shares)$`),
	"price":  regexp.MustCompile(`(?i)^(harga( per lembar)?|price|unit price)$`),
	"gross":  regexp.MustCompile(`(?i)^(bruto|gross|total)( (jual|beli|sell|buy))?$`),
}
var feeLabel = regexp.MustCompile(`(?i)^(komisi\b.*|broker fee|commission|pph|bea materai|bea meterai|stamp duty|levy|ppn)$`)
var netLabel = regexp.MustCompile(`(?i)^(total terima|total bayar|net proceeds|net payable)$`)
var grossLabel = regexp.MustCompile(`(?i)^(total jual|total beli|total transaksi|total gross)$`)
var sideWord = regexp.MustCompile(`(?i)\b(jual|penjualan|sell|beli|pembelian|buy)\b`)
var errAmbiguousCells = errors.New("Tabel Ajaib belum dapat dicocokkan secara pasti dengan sel PDF.")

type ajaibTable struct {
	headers                []string
	rows                   [][]string
	fees                   []dto.ReviewFee
	date, side, net, gross string
	netLine                int
	netLabel               string
	refColumn              int
}

func parseAjaibTable(layout string) (ajaibTable, error) {
	var t ajaibTable
	layout, err := flattenAjaibPanels(layout)
	if err != nil {
		return t, err
	}
	summary := false
	for n, line := range strings.Split(layout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cells := cellGap.Split(line, -1)
		if m := ajaibDate.FindStringSubmatch(line); m != nil {
			if t.date != "" {
				return t, errAmbiguousCells
			}
			for _, format := range []string{"2 January 2006", "2006-01-02"} {
				if d, e := time.Parse(format, m[1]); e == nil {
					t.date = d.Format("2006-01-02")
					break
				}
			}
			continue
		}
		if len(t.headers) == 0 {
			for _, s := range sideWord.FindAllString(line, -1) {
				side := "BUY"
				switch strings.ToLower(s) {
				case "jual", "penjualan", "sell":
					side = "SELL"
				}
				if t.side != "" && t.side != side {
					return t, errors.New("Label beli dan jual ditemukan sebelum tabel Ajaib; batas tiap sisi belum dikenali.")
				}
				t.side = side
			}
			for _, cell := range cells {
				if columnMeanings["ticker"].MatchString(cell) {
					t.headers = cells
					break
				}
			}
			continue
		}
		label := strings.TrimSpace(strings.TrimSuffix(cells[0], ":"))
		if len(cells) == 2 && (feeLabel.MatchString(label) || netLabel.MatchString(label) || grossLabel.MatchString(label)) {
			summary = true
			amount := strings.TrimSpace(strings.TrimPrefix(cells[1], "Rp "))
			if strings.HasPrefix(amount, "(") && strings.HasSuffix(amount, ")") && feeLabel.MatchString(label) {
				amount = amount[1 : len(amount)-1]
			}
			if _, err := cents(amount); err != nil {
				return t, errAmbiguousCells
			}
			switch {
			case feeLabel.MatchString(label):
				fee := dto.ReviewFee{Label: label}
				if t.side == "SELL" {
					fee.Sell = amount
				} else {
					fee.Buy = amount
				}
				t.fees = append(t.fees, fee)
			case netLabel.MatchString(label):
				sell := strings.EqualFold(label, "total terima") || strings.EqualFold(label, "net proceeds")
				if t.net != "" || sell != (t.side == "SELL") {
					return t, errAmbiguousCells
				}
				t.netLabel = label
				t.net = amount
				t.netLine = n + 1
			default:
				if t.gross != "" {
					return t, errAmbiguousCells
				}
				t.gross = amount
			}
			continue
		}
		if summary || len(cells) != len(t.headers) {
			return t, fmt.Errorf("Baris layout %d belum dikenali (header %d sel, baris %d sel, sesudah subtotal=%t).", n+1, len(t.headers), len(cells), summary)
		}
		t.rows = append(t.rows, cells)
	}
	if len(t.rows) == 0 || len(t.rows) > 200 || len(t.headers) < 4 || len(t.headers) > 6 || t.net == "" || t.side == "" || t.date == "" {
		return t, fmt.Errorf("Struktur TC Ajaib belum lengkap: kolom=%d, baris=%d, tanggal=%t, sisi=%t, neto=%t.", len(t.headers), len(t.rows), t.date != "", t.side != "", t.net != "")
	}
	return t, nil
}

type ajaibLocations struct {
	Ticker, Price, Shares, Gross, Net int
}

// Known labels bind their own role. One unfamiliar label can be proposed by the
// model if every other role is anchored; the row arithmetic and printed net
// must still reconcile. Two unanchored roles could exchange price and quantity.
func (t ajaibTable) candidates() (map[string][]int, error) {
	out := make(map[string][]int)
	var unlabelled []int
	for i, label := range t.headers {
		found := false
		for name, meaning := range columnMeanings {
			if meaning.MatchString(label) {
				out[name] = append(out[name], i+1)
				found = true
			}
		}
		switch strings.ToLower(label) {
		case "ref", "reference", "referensi", "board", "tipe":
			found = true
		}
		if !found {
			unlabelled = append(unlabelled, i+1)
		}
	}
	if len(unlabelled) > 1 {
		return nil, errAmbiguousCells
	}
	for name := range columnMeanings {
		if len(out[name]) == 0 && len(unlabelled) == 1 && len(out) == 3 && (name == "price" || name == "shares" || name == "gross") {
			out[name] = unlabelled
		}
		if len(out[name]) != 1 {
			return nil, errAmbiguousCells
		}
	}
	return out, nil
}

func (t ajaibTable) draft(p ajaibLocations) (*dto.TradeDraft, error) {
	if p.Net != t.netLine {
		return nil, errAmbiguousCells
	}
	candidates, err := t.candidates()
	if err != nil {
		return nil, err
	}
	used := map[int]bool{}
	for name, col := range map[string]int{"ticker": p.Ticker, "price": p.Price, "shares": p.Shares, "gross": p.Gross} {
		if col < 1 || col > len(t.headers) || used[col] || candidates[name][0] != col {
			return nil, errAmbiguousCells
		}
		used[col] = true
	}
	for i, label := range t.headers {
		if used[i+1] {
			continue
		}
		switch strings.ToLower(label) {
		case "ref", "reference", "referensi":
			if t.refColumn != 0 {
				return nil, errAmbiguousCells
			}
			t.refColumn = i + 1
		case "tipe", "board":
		default:
			return nil, errAmbiguousCells
		}
	}
	d := &dto.TradeDraft{Date: t.date, Fees: t.fees}
	for _, cells := range t.rows {
		ref := ""
		if t.refColumn > 0 {
			ref = cells[t.refColumn-1]
		}
		d.Rows = append(d.Rows, dto.ReviewFill{Ref: ref, Ticker: cells[p.Ticker-1], Side: t.side, Price: cells[p.Price-1], Shares: cells[p.Shares-1], Gross: cells[p.Gross-1]})
	}
	gross := t.gross
	if gross == "" {
		if len(d.Rows) != 1 {
			return nil, errAmbiguousCells
		}
		gross = d.Rows[0].Gross
	}
	if t.side == "SELL" {
		d.SellGross = gross
		d.SellNet = t.net
	} else {
		d.BuyGross = gross
		d.BuyNet = t.net
	}
	return d, nil
}

func extractAjaibCells(ctx context.Context, layout string) (*dto.TradeDraft, error) {
	table, err := parseAjaibTable(layout)
	if err != nil {
		return nil, err
	}
	var index strings.Builder
	for i, h := range table.headers {
		fmt.Fprintf(&index, "Column %d: %s\n", i+1, h)
	}

	fmt.Fprintf(&index, "Line %d: printed net settlement (%s)\n", table.netLine, table.netLabel)
	props := map[string]any{}
	required := []string{"ticker", "shares", "price", "gross", "net"}
	candidates, err := table.candidates()
	if err != nil {
		return nil, err
	}
	for key, cols := range candidates {
		if !columnMeanings[key].MatchString(table.headers[cols[0]-1]) {
			cols = append([]int{0}, cols...)
		}
		props[key] = map[string]any{"type": "integer", "enum": cols}
	}
	props["net"] = map[string]any{"type": "integer", "enum": []int{table.netLine}}
	schema := map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	prompt := "Return only column numbers: ticker=stock code, shares=quantity, price=unit price, gross=gross trade value. net is the printed net settlement line number. Index numbers start at 1. Return zero for any unfamiliar label whose meaning cannot be determined."
	content, err := askLocalModel(ctx, schema, prompt, index.String())
	if err != nil {
		return nil, err
	}
	var locations ajaibLocations
	if err := json.Unmarshal([]byte(content), &locations); err != nil {
		return nil, errAmbiguousCells
	}
	return table.draft(locations)
}
