package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/dto"
)

const maxInt64 = int64(1<<63 - 1)

var integerPattern = regexp.MustCompile(`^[0-9]+$`)
var moneyPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{2})?$`)
var tickerPattern = regexp.MustCompile(`^[A-Z]{4}$`)

type expectedTrade struct {
	Reference string
	Trade     dto.ParsedTrade
}

// Canonical fixture numbers are ungrouped digits, optionally with two decimal
// places for money. Leading zeroes (including the fixture's "000") are valid.
func cents(s string) (int64, error) {
	if !moneyPattern.MatchString(s) {
		return 0, fmt.Errorf("invalid canonical money")
	}
	parts := strings.Split(s, ".")
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > maxInt64/100 {
		return 0, fmt.Errorf("money out of range")
	}
	fraction := int64(0)
	if len(parts) == 2 {
		fraction, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	if whole*100 > maxInt64-fraction {
		return 0, fmt.Errorf("money out of range")
	}
	return whole*100 + fraction, nil
}

func positiveInteger(s string) (int64, error) {
	if !integerPattern.MatchString(s) {
		return 0, fmt.Errorf("invalid positive integer")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid positive integer")
	}
	return n, nil
}

func loadGroundTruth(r io.Reader) ([]expectedTrade, error) {
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("CSV header: %w", err)
	}
	want := []string{"REF#", "Board", "Share", "Lot", "Quantity", "Price", "Buy", "Sell", "TransactionDate"}
	if len(header) != len(want) {
		return nil, fmt.Errorf("unexpected CSV header")
	}
	for i := range want {
		if header[i] != want[i] {
			return nil, fmt.Errorf("unexpected CSV header at column %d", i+1)
		}
	}
	reader.FieldsPerRecord = len(want)
	var trades []expectedTrade
	for row := 2; ; row++ {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV row %d: invalid record", row)
		}
		trade, err := parseGroundTruthRow(fields)
		if err != nil {
			return nil, fmt.Errorf("CSV row %d: %w", row, err)
		}
		trades = append(trades, trade)
	}
	if len(trades) == 0 {
		return nil, fmt.Errorf("CSV has no transactions")
	}
	return trades, nil
}

func parseGroundTruthRow(f []string) (expectedTrade, error) {
	fail := func(s string) (expectedTrade, error) { return expectedTrade{}, fmt.Errorf("%s", s) }
	if !integerPattern.MatchString(f[0]) {
		return fail("invalid reference")
	}
	if f[1] == "" {
		return fail("missing board")
	}
	share := strings.Fields(f[2])
	if len(share) == 0 || !tickerPattern.MatchString(share[0]) {
		return fail("invalid Share ticker")
	}
	lot, err := positiveInteger(f[3])
	if err != nil {
		return fail("invalid Lot")
	}
	quantity, err := positiveInteger(f[4])
	if err != nil {
		return fail("invalid Quantity")
	}
	if lot > maxInt64/100 || quantity != lot*100 || int64(int(lot)) != lot {
		return fail("Quantity must equal Lot * 100")
	}
	price, err := cents(f[5])
	if err != nil || price <= 0 {
		return fail("invalid Price")
	}
	buy, err := cents(f[6])
	if err != nil {
		return fail("invalid Buy")
	}
	sell, err := cents(f[7])
	if err != nil {
		return fail("invalid Sell")
	}
	if (buy > 0) == (sell > 0) {
		return fail("exactly one of Buy/Sell must be nonzero")
	}
	side, amount := "BUY", buy
	if sell > 0 {
		side, amount = "SELL", sell
	}
	if price > maxInt64/quantity || price*quantity != amount {
		return fail("amount must equal Quantity * Price")
	}
	// DTO uses float64: reject amounts beyond exact integer-cent representation.
	if amount > 1<<53 || price > 1<<53 {
		return fail("money exceeds exact comparison range")
	}
	date, err := time.Parse("02/01/2006", f[8])
	if err != nil || date.Format("02/01/2006") != f[8] {
		return fail("invalid TransactionDate")
	}
	return expectedTrade{Reference: f[0], Trade: dto.ParsedTrade{Ticker: share[0], Lot: int(lot), Price: float64(price) / 100, Type: side, Date: date, AmountDone: float64(amount) / 100}}, nil
}

func moneyEqual(a, b float64) bool {
	return !math.IsNaN(a) && !math.IsNaN(b) && !math.IsInf(a, 0) && !math.IsInf(b, 0) && a >= 0 && b >= 0 && math.Abs(a-b) <= 0.010000001
}

func tradesEqual(a, b dto.ParsedTrade) bool {
	return a.Ticker == b.Ticker && a.Type == b.Type && a.Lot == b.Lot &&
		a.Date.Format("2006-01-02") == b.Date.Format("2006-01-02") && moneyEqual(a.Price, b.Price) && moneyEqual(a.AmountDone, b.AmountDone)
}

// Maximum bipartite matching avoids both duplicate reuse and greedy pairing
// failures when several prices/amounts overlap within the one-cent tolerance.
func compareTrades(want []expectedTrade, got []dto.ParsedTrade) (missing, unexpected int) {
	assigned := make([]int, len(got))
	for i := range assigned {
		assigned[i] = -1
	}
	var match func(int, []bool) bool
	match = func(w int, seen []bool) bool {
		for g := range got {
			if seen[g] || !tradesEqual(want[w].Trade, got[g]) {
				continue
			}
			seen[g] = true
			if assigned[g] == -1 || match(assigned[g], seen) {
				assigned[g] = w
				return true
			}
		}
		return false
	}
	matched := 0
	for w := range want {
		if match(w, make([]bool, len(got))) {
			matched++
		}
	}
	return len(want) - matched, len(got) - matched
}
