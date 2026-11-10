package parser

import (
	"testing"
	"time"
)

func TestStockbitParser_ParseContent_BothSides(t *testing.T) {
	// Sample text corresponding exactly to the user's uploaded image
	sampleContent := `
PT. STOCKBIT SEKURITAS DIGITAL
Trade Confirmation

Transaction Date
06/11/2026

REF #
Board
Share
Lot
Quantity
Price
Buy
Sell
0033568
RG
PART Cipta Perdana Lancar Tbk.
249
24,900.00
98.00
2,440,200.00
0.00
0167201
RG
PART Cipta Perdana Lancar Tbk.
253
25,300.00
97.00
0.00
2,454,100.00

Your RDN Details:
Bank
:
JAGO
Account No.
:
110736694482
Name
:
MARCHEL MATTHEW HOTASI MANULLANG

Total Value
2,440,200.00
2,454,100.00
Total Fee :
0.1500 % All In
Commission
2,345.67
2,359.03
V.A.T Commission
258.02
259.49
IDX Fee
976.08
981.64
V.A.T Levy
80.53
80.99
Income Tax
0
2,454.10
Stamp Duty
0.00
0
Total Cost
2,443,860.30
2,447,964.75

Payment due to you IDR
0.00
4,104.45
`

	p := NewStockbitParser()
	trades, err := p.parseContent(sampleContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}

	// Trade 1: BUY PART
	buyTrade := trades[0]
	if buyTrade.RefNo != "0033568" {
		t.Errorf("expected RefNo 0033568, got %s", buyTrade.RefNo)
	}
	if buyTrade.Ticker != "PART" {
		t.Errorf("expected ticker PART, got %s", buyTrade.Ticker)
	}
	if buyTrade.Type != "BUY" {
		t.Errorf("expected type BUY, got %s", buyTrade.Type)
	}
	if buyTrade.Lot != 249 {
		t.Errorf("expected lot 249, got %d", buyTrade.Lot)
	}
	if buyTrade.Price != 98.00 {
		t.Errorf("expected price 98.00, got %f", buyTrade.Price)
	}
	if buyTrade.Tax != 3660.30 {
		t.Errorf("expected tax 3660.30, got %f", buyTrade.Tax)
	}
	// AmountDone: 2,440,200 + 3,660.30 = 2,443,860.30
	if buyTrade.AmountDone != 2443860.30 {
		t.Errorf("expected AmountDone 2443860.30, got %f", buyTrade.AmountDone)
	}
	expectedDate := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	if buyTrade.Date.Year() != expectedDate.Year() || buyTrade.Date.Month() != expectedDate.Month() || buyTrade.Date.Day() != expectedDate.Day() {
		t.Errorf("expected date 2026-11-06, got %v", buyTrade.Date)
	}

	// Trade 2: SELL PART
	sellTrade := trades[1]
	if sellTrade.RefNo != "0167201" {
		t.Errorf("expected RefNo 0167201, got %s", sellTrade.RefNo)
	}
	if sellTrade.Ticker != "PART" {
		t.Errorf("expected ticker PART, got %s", sellTrade.Ticker)
	}
	if sellTrade.Type != "SELL" {
		t.Errorf("expected type SELL, got %s", sellTrade.Type)
	}
	if sellTrade.Lot != 253 {
		t.Errorf("expected lot 253, got %d", sellTrade.Lot)
	}
	if sellTrade.Price != 97.00 {
		t.Errorf("expected price 97.00, got %f", sellTrade.Price)
	}
	if sellTrade.Tax != 6135.25 {
		t.Errorf("expected tax 6135.25, got %f", sellTrade.Tax)
	}
	// AmountDone: 2,454,100 + 6,135.25 = 2,460,235.25
	if sellTrade.AmountDone != 2460235.25 {
		t.Errorf("expected AmountDone 2460235.25, got %f", sellTrade.AmountDone)
	}
}

func TestStockbitParser_ParseContent_SingleBuy(t *testing.T) {
	sampleContent := `
PT. STOCKBIT SEKURITAS DIGITAL
Transaction Date
03/11/2026

REF #
Board
Share
Lot
Quantity
Price
Buy
Sell
0050736
RG
MIDI Midi Utama Indonesia Tbk.
1
100.00
262.00
26,200.00
0.00

Your RDN Details:
Bank
:
JAGO

Total Value
26,200.00
0.00
Total Fee :
0.1500 % All In
Commission
25.19
0.00
V.A.T Commission
2.77
0.00
IDX Fee
10.48
0.00
V.A.T Levy
0.86
0.00
Income Tax
0
0.00
Stamp Duty
0.00
0
Total Cost
26,239.30
0.00
`

	p := NewStockbitParser()
	trades, err := p.parseContent(sampleContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	trade := trades[0]
	if trade.Ticker != "MIDI" || trade.Type != "BUY" || trade.Lot != 1 || trade.Price != 262.00 {
		t.Errorf("unexpected trade details: %+v", trade)
	}
	if trade.Tax != 39.30 {
		t.Errorf("expected tax 39.30, got %f", trade.Tax)
	}
	if trade.AmountDone != 26239.30 {
		t.Errorf("expected AmountDone 26239.30, got %f", trade.AmountDone)
	}
}
