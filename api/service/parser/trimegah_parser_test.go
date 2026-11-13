package parser

import (
	"testing"
	"time"
)

func TestTrimegahParser_SingleBuy_FromUserImage(t *testing.T) {
	// Sample text matching the user's uploaded images (Image 0 & Image 1)
	sampleContent := `
trimegah
PT Trimegah Sekuritas Indonesia Tbk (NPWP : 01.340.544.4-054.000)
Gedung Artha Graha Lt. 18, Jl. Jend. Sudirman Kav 52-53, Senayan, Kebayoran Baru, Jakarta Selatan 12190
REGULAR
KONFIRMASI TRANSAKSI
Tanggal : 30 Oktober 2026
Halaman : 1

MARCHEL MATTHEW HOTASI MANULLANG
LINGKUNGAN II

Nama Nasabah : MARCHEL MATTHEW HOTASI MANULLANG
Kode Nasabah : MARE7ILT
Account Officer/Cabang : TRIMA / VB
SID-Single Investor ID : IDD1103HZO12420
KSEI Sub Rekening Efek : LG001QXQV00188
Mata Uang : IDR
Perdagangan Bursa : ID - IDX
Mekanisme Settlement : Versus Payment (VP)

JENIS TRX    JENIS PASAR    KODE EFEK    NAMA PERUSAHAAN    HARGA    JUMLAH (LEMBAR)    NILAI TRANSAKSI
BELI    RG    MIDI    MIDI UTAMA INDONESIA Tbk, PT    258    4,300    (1,109,400)

Total Transaksi BELI : (1,109,400)
Total Transaksi JUAL : 0
Total Nilai Transaksi : (1,109,400)
Komisi Perantara Perdagangan Efek : 1,382
LEVY : 332
Sub Total : 1,714
Dasar Pengenaan Pajak (PPN) : 1,571
PPN : 189
KPEI : 111
Pajak Penjualan (0.10 %) : 0
E-Meterai : 0
WHT : 0
Total Pembebanan : 2,014
Nilai Transaksi Bersih : (1,111,414)
Tanggal Pembayaran : 03 November 2026
`

	p := NewTrimegahParser()
	meta := &EmailMetadata{
		From:     "trim.operation@trimegah.com",
		Subject:  "TC 30 Oct 2026 - MARCHEL MATTHEW HOTASI MANULLANG ( MARE7ILT )",
		Filename: "TC_MARE7ILT-20261030.pdf",
	}

	trades, err := p.parseContent(sampleContent, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	trade := trades[0]
	if trade.Ticker != "MIDI" {
		t.Errorf("expected ticker MIDI, got %s", trade.Ticker)
	}
	if trade.Type != "BUY" {
		t.Errorf("expected type BUY, got %s", trade.Type)
	}
	if trade.Lot != 43 {
		t.Errorf("expected lot 43, got %d", trade.Lot)
	}
	if trade.Price != 258 {
		t.Errorf("expected price 258, got %f", trade.Price)
	}
	if trade.Tax != 2014.00 {
		t.Errorf("expected tax 2014.00, got %f", trade.Tax)
	}
	// Gross (4,300 * 258 = 1,109,400) + Tax (2,014) = 1,111,414.00
	if trade.AmountDone != 1111414.00 {
		t.Errorf("expected AmountDone 1111414.00, got %f", trade.AmountDone)
	}
	if trade.BrokerName != "Trimegah Sekuritas" {
		t.Errorf("expected broker 'Trimegah Sekuritas', got %s", trade.BrokerName)
	}

	expectedDate := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	if trade.Date.Year() != expectedDate.Year() || trade.Date.Month() != expectedDate.Month() || trade.Date.Day() != expectedDate.Day() {
		t.Errorf("expected date 2026-10-30, got %v", trade.Date)
	}
}

func TestTrimegahParser_MultiLineCells(t *testing.T) {
	// Sample where each table cell is on its own line (typical raw PDF stream extraction)
	sampleContent := `
PT Trimegah Sekuritas Indonesia Tbk
KONFIRMASI TRANSAKSI
Tanggal : 30 Oktober 2026

Kode Nasabah : MARE7ILT

JENIS TRX
JENIS PASAR
KODE EFEK
NAMA PERUSAHAAN
HARGA
JUMLAH (LEMBAR)
NILAI TRANSAKSI

BELI
RG
MIDI
MIDI UTAMA INDONESIA Tbk, PT
258
4,300
(1,109,400)

Total Transaksi BELI : (1,109,400)
Total Pembebanan : 2,014
Nilai Transaksi Bersih : (1,111,414)
`

	p := NewTrimegahParser()
	trades, err := p.parseContent(sampleContent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	trade := trades[0]
	if trade.Ticker != "MIDI" {
		t.Errorf("expected ticker MIDI, got %s", trade.Ticker)
	}
	if trade.Lot != 43 {
		t.Errorf("expected lot 43, got %d", trade.Lot)
	}
	if trade.Price != 258 {
		t.Errorf("expected price 258, got %f", trade.Price)
	}
	if trade.Tax != 2014.00 {
		t.Errorf("expected tax 2014.00, got %f", trade.Tax)
	}
	if trade.AmountDone != 1111414.00 {
		t.Errorf("expected AmountDone 1111414.00, got %f", trade.AmountDone)
	}
}

func TestTrimegahParser_BothBuyAndSell(t *testing.T) {
	sampleContent := `
PT Trimegah Sekuritas Indonesia Tbk
Tanggal : 15 Januari 2026
Kode Nasabah : MARE7ILT

JENIS TRX    JENIS PASAR    KODE EFEK    NAMA PERUSAHAAN    HARGA    JUMLAH (LEMBAR)    NILAI TRANSAKSI
BELI    RG    BBCA    BANK CENTRAL ASIA Tbk, PT    9,000    10,000    (90,000,000)
JUAL    RG    BBRI    BANK RAKYAT INDONESIA Tbk, PT    5,000    20,000    100,000,000

Total Transaksi BELI : (90,000,000)
Total Transaksi JUAL : 100,000,000
Total Nilai Transaksi : 10,000,000
Total Pembebanan : 380,000
Nilai Transaksi Bersih : 9,620,000
`

	p := NewTrimegahParser()
	trades, err := p.parseContent(sampleContent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}

	// Trade 1: BBCA BUY 10,000 lembar = 100 lot @ 9000 = 90,000,000
	t1 := trades[0]
	if t1.Ticker != "BBCA" || t1.Type != "BUY" || t1.Lot != 100 || t1.Price != 9000 {
		t.Errorf("unexpected trade 1: %+v", t1)
	}

	// Trade 2: BBRI SELL 20,000 lembar = 200 lot @ 5000 = 100,000,000
	t2 := trades[1]
	if t2.Ticker != "BBRI" || t2.Type != "SELL" || t2.Lot != 200 || t2.Price != 5000 {
		t.Errorf("unexpected trade 2: %+v", t2)
	}

	// Total gross = 190,000,000. Total fee = 380,000
	// BBCA gross = 90m (47.37% of 380k = 180,000)
	// BBRI gross = 100m (52.63% of 380k = 200,000)
	if t1.Tax != 180000 {
		t.Errorf("expected t1 tax 180000, got %f", t1.Tax)
	}
	if t1.AmountDone != 90180000 {
		t.Errorf("expected t1 AmountDone 90180000, got %f", t1.AmountDone)
	}
	if t2.Tax != 200000 {
		t.Errorf("expected t2 tax 200000, got %f", t2.Tax)
	}
	if t2.AmountDone != 99800000 {
		t.Errorf("expected t2 AmountDone 99800000, got %f", t2.AmountDone)
	}
}
