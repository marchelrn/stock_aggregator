package service

import (
	"errors"
	"fmt"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var reviewMoney = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,2})?$`)
var reviewTicker = regexp.MustCompile(`^[A-Z0-9]{4}(-[A-Z0-9]{1,8})?$`)
var reviewRef = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,100}$`)

func cents(s string) (int64, error) {
	if !reviewMoney.MatchString(s) {
		return 0, errors.New("Isi nominal rupiah dengan maksimal dua angka desimal.")
	}
	a, b, _ := strings.Cut(s, ".")
	return strconv.ParseInt(a+(b + "00")[:2], 10, 64)
}
func whole(s string) (int64, error) {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil || n == 0 {
		return 0, errors.New("Jumlah harus bilangan bulat positif.")
	}
	return int64(n), nil
}
func reviewSideValue(value string, inactive bool) (int64, error) {
	if value == "" && inactive {
		return 0, nil
	}
	return cents(value)
}
func validateTradeDraft(d dto.TradeDraft) ([]*models.Transaction, error) {
	date, err := time.Parse("2006-01-02", d.Date)
	if err != nil || date.After(time.Now().UTC().Add(24*time.Hour)) {
		return nil, errors.New("Periksa tanggal transaksi di PDF.")
	}
	if len(d.Rows) == 0 || len(d.Rows) > 200 || len(d.Fees) > 50 {
		return nil, errors.New("Periksa jumlah baris transaksi dan biaya.")
	}
	gross := map[string]int64{}
	fees := map[string]int64{}
	values := make([]int64, len(d.Rows))
	seen := map[string]bool{}
	trades := make([]*models.Transaction, 0, len(d.Rows))
	for i, r := range d.Rows {
		if !reviewTicker.MatchString(r.Ticker) || (r.Side != "BUY" && r.Side != "SELL") || (r.Ref != "" && !reviewRef.MatchString(r.Ref)) {
			return nil, fmt.Errorf("Periksa kode saham, referensi, dan sisi transaksi pada baris %d.", i+1)
		}
		lots, e1 := whole(r.Lots)
		shares, e2 := whole(r.Shares)
		price, e3 := cents(r.Price)
		amount, e4 := cents(r.Gross)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || price == 0 || amount == 0 || shares != lots*100 || amount%shares != 0 || amount/shares != price {
			return nil, fmt.Errorf("Baris %d: lot, lembar, harga dan nilai bruto tidak cocok.", i+1)
		}
		key := fmt.Sprintf("%s|%s|%s|%d|%d", strings.TrimLeft(r.Ref, "0"), r.Ticker, r.Side, lots, price)
		if seen[key] {
			return nil, fmt.Errorf("Identitas transaksi baris %d muncul dua kali. Periksa referensinya.", i+1)
		}
		seen[key] = true
		gross[r.Side] += amount
		values[i] = amount
		trades = append(trades, &models.Transaction{Ticker: r.Ticker, Type: models.Type(r.Side), Lot: int(lots), Price: float64(price) / 100, RefNo: r.Ref, Date: date, ReviewedFee: true})
	}
	for i, f := range d.Fees {
		if strings.TrimSpace(f.Label) == "" || len(f.Label) > 120 {
			return nil, fmt.Errorf("Periksa nama biaya pada baris %d.", i+1)
		}
		buy, e1 := reviewSideValue(f.Buy, gross["BUY"] == 0)
		sell, e2 := reviewSideValue(f.Sell, gross["SELL"] == 0)
		if e1 != nil || e2 != nil {
			return nil, fmt.Errorf("Periksa nominal biaya pada baris %d. Isi 0 jika tercetak nol.", i+1)
		}
		fees["BUY"] += buy
		fees["SELL"] += sell
	}
	for _, side := range []string{"BUY", "SELL"} {
		rawGross, rawNet := d.BuyGross, d.BuyNet
		net := gross[side] + fees[side]
		if side == "SELL" {
			rawGross, rawNet = d.SellGross, d.SellNet
			net = gross[side] - fees[side]
		}
		printedGross, e1 := reviewSideValue(rawGross, gross[side] == 0)
		printedNet, e2 := reviewSideValue(rawNet, gross[side] == 0)
		if e1 != nil || e2 != nil || printedGross != gross[side] || printedNet != net || (gross[side] > 0 && net <= 0) || (gross[side] == 0 && fees[side] != 0) {
			return nil, fmt.Errorf("Total %s tidak cocok. Periksa seluruh transaksi, biaya, serta total bruto dan neto yang tercetak.", side)
		}
	}
	// Allocate rounded cumulative shares of each side's fees so every cent is accounted for.
	prior := map[string]int64{}
	subtotal := map[string]int64{}
	for i, t := range trades {
		side := string(t.Type)
		subtotal[side] += values[i]
		n := new(big.Int).Mul(big.NewInt(fees[side]), big.NewInt(subtotal[side]))
		n.Add(n, big.NewInt(gross[side]/2))
		allocated := n.Quo(n, big.NewInt(gross[side])).Int64()
		fee := allocated - prior[side]
		prior[side] = allocated
		net := values[i] + fee
		if t.Type == models.TypeSell {
			net = values[i] - fee
		}
		if net <= 0 {
			return nil, errors.New("Nilai transaksi setelah biaya harus positif.")
		}
		t.AmountDone = float64(net) / 100
		t.Tax = float64(fee) / 100
	}
	return trades, nil
}
