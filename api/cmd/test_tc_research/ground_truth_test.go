package main

import (
	"encoding/csv"
	"math"
	"strings"
	"testing"

	"github.com/marchelrn/stock_api/dto"
)

const testCSV = `REF#,Board,Share,Lot,Quantity,Price,Buy,Sell,TransactionDate
0077636,RG,ASSA Adi Sarana Armada Tbk.,44,4400,460.00,2024000.00,0.00,19/10/2026
0312251,RG,BTPS Bank BTPN Syariah Tbk.,34,3400,860.00,000,2924000.00,19/10/2026
`

func TestGroundTruth(t *testing.T) {
	trades, err := loadGroundTruth(strings.NewReader(testCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 2 || trades[0].Reference != "0077636" || trades[1].Reference != "0312251" {
		t.Fatalf("references not preserved: %+v", trades)
	}
	if trades[0].Trade.Ticker != "ASSA" || trades[0].Trade.Type != "BUY" || trades[0].Trade.AmountDone != 2024000 || trades[1].Trade.Type != "SELL" || trades[1].Trade.AmountDone != 2924000 {
		t.Fatalf("incorrect trades: %+v", trades)
	}
	if trades[1].Trade.Date.Format("2006-01-02") != "2026-10-19" {
		t.Fatal("incorrect date")
	}
}

func TestGroundTruthRejectsInvalidRows(t *testing.T) {
	records, err := csv.NewReader(strings.NewReader(testCSV)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		column int
		value  string
	}{
		{"reference", 0, ""}, {"board", 1, ""}, {"share", 2, ""}, {"ticker", 2, "assa Company"},
		{"fractional lot", 3, "44.0"}, {"negative lot", 3, "-44"}, {"zero lot", 3, "0"}, {"lot whitespace", 3, " 44"},
		{"overflow lot", 3, "9223372036854775807"}, {"lot parse overflow", 3, "999999999999999999999"},
		{"quantity", 4, "4399"}, {"fractional quantity", 4, "4400.00"}, {"quantity exponent", 4, "4.4e3"},
		{"price nan", 5, "NaN"}, {"price infinity", 5, "Inf"}, {"price comma", 5, "460,00"},
		{"price grouping", 5, "1,460.00"}, {"price precision", 5, "460.001"}, {"price single decimal", 5, "460.0"},
		{"price negative", 5, "-460.00"}, {"price zero", 5, "0.00"}, {"price overflow", 5, "92233720368547758.08"},
		{"both sides zero", 6, "000"}, {"amount arithmetic", 6, "2024000.01"}, {"amount malformed", 6, "2_024_000.00"},
		{"both sides positive", 7, "1.00"}, {"negative sell", 7, "-1.00"},
		{"invalid date", 8, "31/02/2026"}, {"short date", 8, "9/10/2026"}, {"wrong date format", 8, "2026-10-19"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := append([]string(nil), records[1]...)
			row[tc.column] = tc.value
			if _, err := parseGroundTruthRow(row); err == nil {
				t.Fatal("accepted invalid row")
			}
		})
	}
	for _, input := range []string{"", strings.Join(records[0], ",") + "\n", strings.Replace(testCSV, "TransactionDate", "Date", 1), testCSV + "short,row\n"} {
		if _, err := loadGroundTruth(strings.NewReader(input)); err == nil {
			t.Fatal("accepted invalid CSV")
		}
	}
}

func TestCents(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int64
	}{{"000", 0}, {"0.00", 0}, {"460.00", 46000}, {"0001.01", 101}, {"92233720368547758.07", maxInt64}} {
		got, err := cents(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("cents(%q) = %d, %v", tc.input, got, err)
		}
	}
}

func testTrades(t *testing.T) []expectedTrade {
	t.Helper()
	trades, err := loadGroundTruth(strings.NewReader(testCSV))
	if err != nil {
		t.Fatal(err)
	}
	return trades
}

func TestCompareTrades(t *testing.T) {
	want := testTrades(t)
	a, b := want[0].Trade, want[1].Trade
	tests := []struct {
		name                string
		want                []expectedTrade
		got                 []dto.ParsedTrade
		missing, unexpected int
	}{
		{"unordered", want, []dto.ParsedTrade{b, a}, 0, 0},
		{"duplicate not reused", []expectedTrade{want[0], want[0]}, []dto.ParsedTrade{a, b}, 1, 1},
		{"duplicate preserved", []expectedTrade{want[0], want[0]}, []dto.ParsedTrade{a, a}, 0, 0},
		{"extra duplicate", want, []dto.ParsedTrade{a, b, b}, 0, 1},
		{"missing duplicate", []expectedTrade{want[0], want[0]}, []dto.ParsedTrade{a}, 1, 0},
		{"empty", want, nil, 2, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			missing, unexpected := compareTrades(tc.want, tc.got)
			if missing != tc.missing || unexpected != tc.unexpected {
				t.Fatalf("got (%d,%d), want (%d,%d)", missing, unexpected, tc.missing, tc.unexpected)
			}
		})
	}
	for _, field := range []string{"ticker", "side", "lot", "date", "price", "amount", "NaN", "Inf"} {
		t.Run(field, func(t *testing.T) {
			changed := a
			switch field {
			case "ticker":
				changed.Ticker = "XXXX"
			case "side":
				changed.Type = "SELL"
			case "lot":
				changed.Lot++
			case "date":
				changed.Date = changed.Date.AddDate(0, 0, 1)
			case "price":
				changed.Price += .02
			case "amount":
				changed.AmountDone += .02
			case "NaN":
				changed.Price = math.NaN()
			case "Inf":
				changed.AmountDone = math.Inf(1)
			}
			if tradesEqual(a, changed) {
				t.Fatal("accepted changed field")
			}
		})
	}
	within := a
	within.Price += .01
	within.AmountDone -= .01
	within.BrokerName = "excluded"
	if !tradesEqual(a, within) {
		t.Fatal("cent tolerance or broker exclusion failed")
	}
}

func TestCompareOverlappingToleranceRequiresReassignment(t *testing.T) {
	a := testTrades(t)[0]
	b := a
	a.Trade.Price = 100.01
	b.Trade.Price = 100
	first, second := a.Trade, a.Trade
	first.Price, second.Price = 100, 100.02
	missing, unexpected := compareTrades([]expectedTrade{a, b}, []dto.ParsedTrade{first, second})
	if missing != 0 || unexpected != 0 {
		t.Fatalf("matching must reassign earlier pair: %d, %d", missing, unexpected)
	}
}
