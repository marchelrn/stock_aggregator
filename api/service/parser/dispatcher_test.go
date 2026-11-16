package parser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marchelrn/stock_api/models"
)

func TestBrokerDispatcherParser_EmailSelectorsMerge(t *testing.T) {
	stockbit := EmailMetadata{From: "confirmation@stockbit.com", Subject: "Trade Confirmation", Filename: ".pdf"}
	trimegahTC := EmailMetadata{From: "trim.operation@trimegah.com", Subject: "TC"}
	trimegahConfirmation := EmailMetadata{From: "trim.operation@trimegah.com", Subject: "Trade Confirmation"}
	for _, duplicate := range []bool{false, true} {
		name := "Stockbit templates only"
		input := []EmailMetadata{stockbit}
		want := []EmailMetadata{stockbit, trimegahTC, trimegahConfirmation}
		if duplicate {
			name = "deduplicate full tuples"
			withFilename := trimegahTC
			withFilename.Filename = ".pdf"
			input = []EmailMetadata{stockbit, stockbit, trimegahTC, withFilename}
			want = []EmailMetadata{stockbit, trimegahTC, withFilename, trimegahConfirmation}
		}
		t.Run(name, func(t *testing.T) {
			store := &metadataFakeStore{}
			for i, selector := range input {
				template := metadataTestTemplate(t, metadataTestConfig())
				template.Version = i + 1
				template.EmailSender = selector.From
				template.SubjectContains = selector.Subject
				template.FilenameSuffix = selector.Filename
				store.templates = append(store.templates, template)
			}
			dispatcher := NewBrokerDispatcherParser(NewMetadataParser(store), NewStockbitParser(), NewMandiriParser(), NewTrimegahParser())
			selectors, err := dispatcher.EmailSelectors(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(selectors) != len(want) {
				t.Fatalf("expected selectors %+v, got %+v", want, selectors)
			}
			for i := range want {
				if selectors[i] != want[i] {
					t.Errorf("selector %d: expected %+v, got %+v", i, want[i], selectors[i])
				}
			}
		})
	}
}

func TestBrokerDispatcherParser_EmailSelectorsErrors(t *testing.T) {
	storeErr := errors.New("template store unavailable")
	invalid := metadataTestTemplate(t, metadataTestConfig())
	invalid.Config = []byte(`{}`)
	for name, metadata := range map[string]*MetadataParser{
		"nil store":           NewMetadataParser(nil),
		"store error":         NewMetadataParser(&metadataFakeStore{err: storeErr}),
		"no active templates": NewMetadataParser(&metadataFakeStore{}),
		"invalid template":    NewMetadataParser(&metadataFakeStore{templates: []models.PDFParsingTemplate{invalid}}),
	} {
		t.Run(name, func(t *testing.T) {
			dispatcher := NewBrokerDispatcherParser(metadata, NewStockbitParser(), NewMandiriParser(), NewTrimegahParser())
			selectors, err := dispatcher.EmailSelectors(context.Background())
			if err == nil || selectors != nil {
				t.Fatalf("expected metadata error without fallback selectors, got %+v, %v", selectors, err)
			}
			if name == "store error" && !errors.Is(err, storeErr) {
				t.Errorf("expected original store error, got %v", err)
			}
		})
	}
}

// TC_EIPO files carry a "Buy/Refund" table with the ticker attached to the
// company name; they must be routed to the EIPO parser, not the regular
// Stockbit parser (which rejects them as inapplicable).
func TestBrokerDispatcherParser_EIPORouting(t *testing.T) {
	metadata := NewMetadataParser(nil)
	dispatcher := NewBrokerDispatcherParser(metadata, NewStockbitParser(), NewMandiriParser(), NewTrimegahParser())

	pdf := metadataTestPDF(`Trade Confirmation
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
CDIA PT Chandra Daya Investasi Tbk
11
1,100
190
209,000
0`)

	meta := EmailMetadata{
		From:     "confirmation@stockbit.com",
		Subject:  "Trade Confirmation - IPO",
		Filename: "TC_EIPO_CDIA_1789262_20250710.pdf",
	}

	trades, err := dispatcher.ParseEmail(context.Background(), pdf, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 EIPO trade, got %d: %+v", len(trades), trades)
	}
	if tr := trades[0]; tr.Ticker != "CDIA" || tr.Type != "BUY" || tr.Lot != 11 || tr.Price != 190 {
		t.Errorf("unexpected trade: %+v", tr)
	}
}

func TestBrokerDispatcherParser_StockbitTCFilename(t *testing.T) {
	pdf := metadataTestPDF(`Trade Confirmation
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
0.00`)
	for _, from := range []string{"confirmation@stockbit.com", "confirmation@example.test"} {
		t.Run(from, func(t *testing.T) {
			dispatcher := NewBrokerDispatcherParser(NewMetadataParser(nil), NewStockbitParser(), NewMandiriParser(), NewTrimegahParser())
			meta := EmailMetadata{From: from, Subject: "TC 06 Nov 2026", Filename: "TC_TEST001-20261106.pdf"}
			trades, err := dispatcher.ParseEmail(context.Background(), pdf, meta)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(trades) != 1 {
				t.Fatalf("expected 1 trade, got %d: %+v", len(trades), trades)
			}
			if trade := trades[0]; trade.BrokerName != "Stockbit Sekuritas" || trade.Ticker != "PART" || trade.Type != "BUY" || trade.Lot != 249 || trade.Price != 98 {
				t.Errorf("unexpected trade: %+v", trade)
			}
		})
	}
}

func TestBrokerDispatcherParser_EIPORefundOnly(t *testing.T) {
	pdf := metadataTestPDF(`Trade Confirmation
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
TEST001
IPO
JELI PT Jaya Guna Elektronik Tbk
5
500
200
0
100,000`)
	dispatcher := NewBrokerDispatcherParser(NewMetadataParser(nil), NewStockbitParser(), NewMandiriParser(), NewTrimegahParser())
	meta := EmailMetadata{From: "confirmation@stockbit.com", Subject: "Trade Confirmation - IPO", Filename: "TC_EIPO_JELI_TEST001.pdf"}
	trades, err := dispatcher.ParseEmail(context.Background(), pdf, meta)
	if !errors.Is(err, ErrEmailNotApplicable) || len(trades) != 0 {
		t.Fatalf("expected refund-only skip, got trades=%+v, err=%v", trades, err)
	}
}

func TestBrokerDispatcherParser_TrimegahPDFRouting(t *testing.T) {
	content := `KONFIRMASI TRANSAKSI
Tanggal : 30 Oktober 2026
Kode Nasabah : TEST001
BELI RG MIDI MIDI UTAMA INDONESIA Tbk, PT 258 4,300 (1,109,400)
Total Transaksi BELI : (1,109,400)
Total Pembebanan : 2,014
Nilai Transaksi Bersih : (1,111,414)`

	tests := []struct {
		name    string
		meta    EmailMetadata
		content string
	}{
		{
			name:    "sender with TC filename",
			meta:    EmailMetadata{From: "trim.operation@trimegah.com", Subject: "TC 30 Oct 2026", Filename: "TC_TEST001-20261030.pdf"},
			content: content,
		},
		{
			name:    "sender wins over Stockbit skip marker",
			meta:    EmailMetadata{From: "trim.operation@trimegah.com", Subject: "TC 30 Oct 2026", Filename: "TC_TEST001-20261030.pdf"},
			content: content + "\nRefund : 0",
		},
		{
			name:    "sender wins over EIPO filename heuristic",
			meta:    EmailMetadata{From: "TRIM.OPERATION@TRIMEGAH.COM", Subject: "Trade Confirmation", Filename: "TC_EIPO001-20261030.pdf"},
			content: content,
		},
		{
			name:    "sender alias wins over IPO subject heuristic",
			meta:    EmailMetadata{From: "trim.operation@example.test", Subject: "TC - IPO TEST", Filename: "TC_TEST001-20261030.pdf"},
			content: content,
		},
		{
			name:    "broker subject wins over generic filename",
			meta:    EmailMetadata{Subject: "TRIMEGAH Trade Confirmation", Filename: "TC_TEST001-20261030.pdf"},
			content: content + "\nRefund : 0",
		},
		{
			name:    "broker filename wins over generic filename",
			meta:    EmailMetadata{Filename: "TC_TRIMEGAH_TEST001.pdf"},
			content: content + "\nRefund : 0",
		},
		{
			name:    "broker content wins over generic filename",
			meta:    EmailMetadata{Filename: "TC_TEST001-20261030.pdf"},
			content: "PT TRIMEGAH SEKURITAS\n" + content + "\nRefund : 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatcher := NewBrokerDispatcherParser(NewMetadataParser(nil), NewStockbitParser(), NewMandiriParser(), NewTrimegahParser())
			trades, err := dispatcher.ParseEmail(context.Background(), metadataTestPDF(tt.content), tt.meta)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(trades) != 1 {
				t.Fatalf("expected 1 trade, got %d: %+v", len(trades), trades)
			}
			trade := trades[0]
			if trade.BrokerName != "Trimegah Sekuritas" || trade.Ticker != "MIDI" || trade.Type != "BUY" || trade.Lot != 43 || trade.Price != 258 || trade.AmountDone != 1111414 {
				t.Errorf("unexpected trade: %+v", trade)
			}
		})
	}
}

func TestBrokerDispatcherParser_TrimegahErrors(t *testing.T) {
	tests := []struct {
		name          string
		data          []byte
		wantError     string
		notApplicable bool
	}{
		{name: "invalid PDF", data: []byte("not a PDF"), wantError: "gagal membaca PDF Trimegah"},
		{name: "no trades", data: metadataTestPDF("KONFIRMASI TRANSAKSI\nTanggal : 30 Oktober 2026"), wantError: "tidak ada transaksi ditemukan pada PDF Trimegah"},
		{name: "empty PDF", data: metadataTestPDF(""), notApplicable: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatcher := NewBrokerDispatcherParser(NewMetadataParser(nil), NewStockbitParser(), NewMandiriParser(), NewTrimegahParser())
			meta := EmailMetadata{From: "trim.operation@trimegah.com", Subject: "TC 30 Oct 2026", Filename: "TC_TEST001-20261030.pdf"}
			trades, err := dispatcher.ParseEmail(context.Background(), tt.data, meta)
			if err == nil || len(trades) != 0 {
				t.Fatalf("expected an error and no trades, got trades=%+v, err=%v", trades, err)
			}
			if errors.Is(err, ErrEmailNotApplicable) != tt.notApplicable {
				t.Errorf("expected not-applicable=%v, got %v", tt.notApplicable, err)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestBrokerDispatcherParser_TrimegahRouting(t *testing.T) {
	var metadata *MetadataParser
	stockbit := NewStockbitParser()
	mandiri := NewMandiriParser()
	trimegah := NewTrimegahParser()

	dispatcher := NewBrokerDispatcherParser(metadata, stockbit, mandiri, trimegah)

	sampleContent := `
PT Trimegah Sekuritas Indonesia Tbk
Tanggal : 30 Oktober 2026
Kode Nasabah : MARE7ILT

JENIS TRX    JENIS PASAR    KODE EFEK    NAMA PERUSAHAAN    HARGA    JUMLAH (LEMBAR)    NILAI TRANSAKSI
BELI    RG    MIDI    MIDI UTAMA INDONESIA Tbk, PT    258    4,300    (1,109,400)

Total Transaksi BELI : (1,109,400)
Total Pembebanan : 2,014
Nilai Transaksi Bersih : (1,111,414)
`

	meta := EmailMetadata{
		From:     "trim.operation@trimegah.com",
		Subject:  "TC 30 Oct 2026 - MARCHEL MATTHEW HOTASI MANULLANG ( MARE7ILT )",
		Filename: "TC_MARE7ILT-20261030.pdf",
	}

	// Test ParseEmail routing
	trades, err := dispatcher.ParseEmail(context.Background(), []byte(sampleContent), meta)
	if err != nil {
		t.Fatalf("unexpected ParseEmail error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if trades[0].BrokerName != "Trimegah Sekuritas" {
		t.Errorf("expected broker 'Trimegah Sekuritas', got %s", trades[0].BrokerName)
	}
	if trades[0].Ticker != "MIDI" {
		t.Errorf("expected ticker 'MIDI', got %s", trades[0].Ticker)
	}
	if trades[0].Lot != 43 {
		t.Errorf("expected 43 lot, got %d", trades[0].Lot)
	}
	if trades[0].AmountDone != 1111414.00 {
		t.Errorf("expected AmountDone 1111414.00, got %f", trades[0].AmountDone)
	}

	// Test EmailSelectors includes Trimegah
	selectors, err := dispatcher.EmailSelectors(context.Background())
	if err != nil {
		t.Fatalf("unexpected EmailSelectors error: %v", err)
	}

	foundTrimegah := false
	for _, s := range selectors {
		if s.From == "trim.operation@trimegah.com" {
			foundTrimegah = true
			break
		}
	}
	if !foundTrimegah {
		t.Errorf("expected trim.operation@trimegah.com in EmailSelectors, got: %+v", selectors)
	}
}

func TestBrokerDispatcherParser_AnotherUser_TrimegahRouting(t *testing.T) {
	metadata := NewMetadataParser(nil)
	stockbit := NewStockbitParser()
	mandiri := NewMandiriParser()
	trimegah := NewTrimegahParser()

	dispatcher := NewBrokerDispatcherParser(metadata, stockbit, mandiri, trimegah)

	// Completely different user, code, ticker, and date
	sampleContent := `
PT Trimegah Sekuritas Indonesia Tbk
Tanggal : 12 November 2026
Kode Nasabah : BPRA001

JENIS TRX    JENIS PASAR    KODE EFEK    NAMA PERUSAHAAN    HARGA    JUMLAH (LEMBAR)    NILAI TRANSAKSI
BELI    RG    BBCA    BANK CENTRAL ASIA Tbk, PT    9,500    5,000    (47,500,000)

Total Transaksi BELI : (47,500,000)
Total Pembebanan : 95,000
Nilai Transaksi Bersih : (47,595,000)
`

	meta := EmailMetadata{
		From:     "trim.operation@trimegah.com",
		Subject:  "TC 12 Nov 2026 - BUDI PRATAMA ( BPRA001 )",
		Filename: "TC_BPRA001-20261112.pdf",
	}

	trades, err := dispatcher.ParseEmail(context.Background(), []byte(sampleContent), meta)
	if err != nil {
		t.Fatalf("unexpected ParseEmail error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if trades[0].BrokerName != "Trimegah Sekuritas" {
		t.Errorf("expected broker 'Trimegah Sekuritas', got %s", trades[0].BrokerName)
	}
	if trades[0].Ticker != "BBCA" {
		t.Errorf("expected ticker 'BBCA', got %s", trades[0].Ticker)
	}
	if trades[0].Lot != 50 { // 5,000 shares / 100 = 50 lot
		t.Errorf("expected 50 lot, got %d", trades[0].Lot)
	}
	if trades[0].Price != 9500 {
		t.Errorf("expected price 9500, got %f", trades[0].Price)
	}
	if trades[0].AmountDone != 47595000.00 {
		t.Errorf("expected AmountDone 47595000.00, got %f", trades[0].AmountDone)
	}
}

// TestBrokerDispatcherParser_TrimegahMultipleTrades tests Trimegah extraction with multiple trades
func TestBrokerDispatcherParser_TrimegahMultipleTrades(t *testing.T) {
	metadata := NewMetadataParser(nil)
	stockbit := NewStockbitParser()
	mandiri := NewMandiriParser()
	trimegah := NewTrimegahParser()

	dispatcher := NewBrokerDispatcherParser(metadata, stockbit, mandiri, trimegah)

	sampleContent := `
PT Trimegah Sekuritas Indonesia Tbk
Tanggal : 15 November 2026
Kode Nasabah : TEST001

JENIS TRX    JENIS PASAR    KODE EFEK    NAMA PERUSAHAAN    HARGA    JUMLAH (LEMBAR)    NILAI TRANSAKSI
BELI    RG    ASII    ASTRA INTERNATIONAL Tbk, PT    6850    1,000    (6,850,000)
JUAL    RG    UNTR    UNITED TRACTORS Tbk, PT    28900    500    14,450,000

Total Transaksi BELI : (6,850,000)
Total Transaksi JUAL : 14,450,000
Total Pembebanan : 25,000
Total Penerimaan : 14,425,000
`

	meta := EmailMetadata{
		From:     "trim.operation@trimegah.com",
		Subject:  "TC 15 Nov 2026 - TEST USER ( TEST001 )",
		Filename: "TC_TEST001-20261115.pdf",
	}

	trades, err := dispatcher.ParseEmail(context.Background(), []byte(sampleContent), meta)
	if err != nil {
		t.Fatalf("unexpected ParseEmail error: %v", err)
	}

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}

	// Verify first trade (BELI ASII)
	if trades[0].Ticker != "ASII" {
		t.Errorf("expected first ticker 'ASII', got %s", trades[0].Ticker)
	}
	if trades[0].Type != "BUY" {
		t.Errorf("expected first type 'BUY', got %s", trades[0].Type)
	}
	if trades[0].Lot != 10 { // 1,000 shares / 100 = 10 lot
		t.Errorf("expected first lot 10, got %d", trades[0].Lot)
	}
	if trades[0].Price != 6850 {
		t.Errorf("expected first price 6850, got %f", trades[0].Price)
	}
	// AmountDone includes tax/fee
	if trades[0].AmountDone <= 0 {
		t.Errorf("expected first AmountDone > 0, got %f", trades[0].AmountDone)
	}

	// Verify second trade (JUAL UNTR)
	if trades[1].Ticker != "UNTR" {
		t.Errorf("expected second ticker 'UNTR', got %s", trades[1].Ticker)
	}
	if trades[1].Type != "SELL" {
		t.Errorf("expected second type 'SELL', got %s", trades[1].Type)
	}
	if trades[1].Lot != 5 { // 500 shares / 100 = 5 lot
		t.Errorf("expected second lot 5, got %d", trades[1].Lot)
	}
	if trades[1].Price != 28900 {
		t.Errorf("expected second price 28900, got %f", trades[1].Price)
	}
	if trades[1].AmountDone <= 0 {
		t.Errorf("expected second AmountDone > 0, got %f", trades[1].AmountDone)
	}

	// Verify all trades have correct broker name
	for i, trade := range trades {
		if trade.BrokerName != "Trimegah Sekuritas" {
			t.Errorf("trade %d: expected broker 'Trimegah Sekuritas', got %s", i, trade.BrokerName)
		}
	}
}

// TestBrokerDispatcherParser_TrimegahPriceFormatting tests various price formats in Trimegah TC
func TestBrokerDispatcherParser_TrimegahPriceFormatting(t *testing.T) {
	metadata := NewMetadataParser(nil)
	stockbit := NewStockbitParser()
	mandiri := NewMandiriParser()
	trimegah := NewTrimegahParser()

	dispatcher := NewBrokerDispatcherParser(metadata, stockbit, mandiri, trimegah)

	// Test with standard comma/dot format (no ambiguity)
	sampleContent := `
PT Trimegah Sekuritas Indonesia Tbk
Tanggal : 20 November 2026
Kode Nasabah : EURO001

JENIS TRX    JENIS PASAR    KODE EFEK    NAMA PERUSAHAAN    HARGA    JUMLAH (LEMBAR)    NILAI TRANSAKSI
BELI    RG    INCO    VALE INDONESIA Tbk, PT    3500    2,000    (7,000,000)

Total Transaksi BELI : (7,000,000)
Total Pembebanan : 15,000
Nilai Transaksi Bersih : (7,015,000)
`

	meta := EmailMetadata{
		From:     "trim.operation@trimegah.com",
		Subject:  "TC 20 Nov 2026 - EURO USER ( EURO001 )",
		Filename: "TC_EURO001-20261120.pdf",
	}

	trades, err := dispatcher.ParseEmail(context.Background(), []byte(sampleContent), meta)
	if err != nil {
		t.Fatalf("unexpected ParseEmail error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	// Verify price parsing
	if trades[0].Price != 3500 {
		t.Errorf("expected price 3500, got %f", trades[0].Price)
	}
	if trades[0].Lot != 20 { // 2,000 shares / 100 = 20 lot
		t.Errorf("expected lot 20, got %d", trades[0].Lot)
	}
	// Amount with tax/fee included
	if trades[0].AmountDone < 7015000.00 {
		t.Errorf("expected AmountDone >= 7015000.00, got %f", trades[0].AmountDone)
	}
}

// TestBrokerDispatcherParser_TrimegahDispatcherRouting tests that dispatcher correctly routes Trimegah emails
func TestBrokerDispatcherParser_TrimegahDispatcherRouting(t *testing.T) {
	metadata := NewMetadataParser(nil)
	stockbit := NewStockbitParser()
	mandiri := NewMandiriParser()
	trimegah := NewTrimegahParser()

	dispatcher := NewBrokerDispatcherParser(metadata, stockbit, mandiri, trimegah)

	testCases := []struct {
		name        string
		from        string
		subject     string
		filename    string
		shouldRoute bool
	}{
		{
			name:        "Standard Trimegah TC email",
			from:        "trim.operation@trimegah.com",
			subject:     "TC 25 Nov 2026 - TEST USER ( TEST001 )",
			filename:    "TC_TEST001-20261125.pdf",
			shouldRoute: true,
		},
		{
			name:        "Trimegah in from field",
			from:        "noreply@trimegah.com",
			subject:     "Trade Confirmation",
			filename:    "trade.pdf",
			shouldRoute: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sampleContent := `PT Trimegah Sekuritas Indonesia Tbk
Tanggal : 25 November 2026
Kode Nasabah : TEST001

JENIS TRX    JENIS PASAR    KODE EFEK    NAMA PERUSAHAAN    HARGA    JUMLAH (LEMBAR)    NILAI TRANSAKSI
BELI    RG    TLKM    TELEKOMUNIKASI INDONESIA Tbk, PT    3.250    1,500    (4.875.000)

Total Transaksi BELI : (4.875.000)
Total Pembebanan : 10.000
Nilai Transaksi Bersih : (4.885.000)
`

			meta := EmailMetadata{
				From:     tc.from,
				Subject:  tc.subject,
				Filename: tc.filename,
			}

			trades, err := dispatcher.ParseEmail(context.Background(), []byte(sampleContent), meta)

			if tc.shouldRoute {
				if err != nil {
					t.Errorf("expected successful routing, got error: %v", err)
				}
				if len(trades) == 0 {
					t.Errorf("expected trades to be parsed, got 0")
				}
			}
		})
	}
}
