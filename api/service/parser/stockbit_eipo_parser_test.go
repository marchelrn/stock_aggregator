package parser

import (
	"errors"
	"testing"
	"time"
)

// The daily TC_EIPO PDFs put every table cell on its own line and prefix the
// ticker onto the company name ("CDIA PT Chandra Daya Investasi Tbk").
const eipoMultilineContent = `Trade Confirmation
Allotment Date
07/07/2025
Distribution Date
08/07/2025
This is to confirm the following activity on your account:
REF #
Board
Share
Lot
Quantity
Price
Buy
Refund
MP9WSLPZK8
IPO
CDIA PT Chandra Daya Investasi Tbk
11
1,100
190
209,000
0
MP9WSLPZK8
IPO
CDIA PT Chandra Daya Investasi Tbk
16
1,600
190
0
304,000
Your RDN Details:
`

func TestStockbitEIPOParser_MultilineAllotment(t *testing.T) {
	p := NewStockbitEIPOParser()
	trades, err := p.parseContent(eipoMultilineContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 allotted trade (refund row excluded), got %d: %+v", len(trades), trades)
	}
	tr := trades[0]
	if tr.Ticker != "CDIA" || tr.Type != "BUY" || tr.Lot != 11 || tr.Price != 190 {
		t.Errorf("unexpected trade: %+v", tr)
	}
	if tr.RefNo != "MP9WSLPZK8" {
		t.Errorf("expected RefNo MP9WSLPZK8, got %q", tr.RefNo)
	}
	if want := time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC); !tr.Date.Equal(want) {
		t.Errorf("expected allotment date %v, got %v", want, tr.Date)
	}
	// Must settle into the regular Stockbit broker, not a separate one.
	if tr.BrokerName != "Stockbit Sekuritas" {
		t.Errorf("expected broker Stockbit Sekuritas, got %q", tr.BrokerName)
	}
}

func TestStockbitEIPOParser_RefundOnlyIsNotApplicable(t *testing.T) {
	content := `Trade Confirmation
Allotment Date
07/07/2025
REF #
Board
Share
Lot
Quantity
Price
Buy
Refund
MP9WSLPZK8
IPO
JELI PT Jaya Guna Elektronik Tbk
5
500
200
0
100,000
Your RDN Details:
`
	p := NewStockbitEIPOParser()
	_, err := p.parseContent(content)
	if !errors.Is(err, ErrEmailNotApplicable) {
		t.Fatalf("expected ErrEmailNotApplicable for refund-only PDF, got %v", err)
	}
}

func TestStockbitEIPOParser_SingleLineRow(t *testing.T) {
	content := `Trade Confirmation
Allotment Date
15/08/2025
REF # Board Share Lot Quantity Price Buy Refund
6374143 IPO PRDL PT Prodia Diagnostic Line Tbk 1 100 120 12,000 0
Your RDN Details:
`
	p := NewStockbitEIPOParser()
	trades, err := p.parseContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	tr := trades[0]
	if tr.Ticker != "PRDL" || tr.Lot != 1 || tr.Price != 120 || tr.RefNo != "6374143" {
		t.Errorf("unexpected trade: %+v", tr)
	}
}

func TestStockbitEIPOParser_MissingAllotmentDateIsNotApplicable(t *testing.T) {
	p := NewStockbitEIPOParser()
	_, err := p.parseContent("some unrelated trade confirmation text")
	if !errors.Is(err, ErrEmailNotApplicable) {
		t.Fatalf("expected ErrEmailNotApplicable, got %v", err)
	}
}
