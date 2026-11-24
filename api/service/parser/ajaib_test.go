package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/marchelrn/stock_api/models"
)

func TestAjaibColumns(t *testing.T) {
	var seed models.PDFParsingTemplate
	data, err := os.ReadFile("../../migrations/templates/ajaib_trade.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &seed); err != nil {
		t.Fatal(err)
	}
	seed.Active = true
	c, err := compileMetadata(seed)
	if err != nil {
		t.Fatal(err)
	}
	layout := "Tgl. Transaksi : 16 October 2026\n" + fmt.Sprintf("%-52s%s\n", "Tipe Efek Harga Jumlah Total Beli", "Tipe Efek Harga Jumlah Total Jual") + fmt.Sprintf("%-52s%s\n", "RG AAAA 100 200 20,000", "RG BBBB 300 100 30,000") + "Total Transaksi : 10,000\nKomisi Broker Levy, PPN : (50)\nPPH : (30)\n"
	trades, err := c.parseGenericTrades(layout, nil)
	if err != nil || len(trades) != 2 {
		t.Fatalf("rows=%d err=%v", len(trades), err)
	}
	if trades[0].Type != "BUY" || trades[1].Type != "SELL" || trades[0].Tax != 20 || trades[1].Tax != 60 {
		t.Fatalf("incorrect column or fee allocation: %+v", trades)
	}
	if path := os.Getenv("AJAIB_SAMPLE_PDF"); path != "" {
		pdf, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		dispatcher := NewBrokerDispatcherParser(NewMetadataParser(attachmentTemplates{seed}), nil, nil, nil)
		trades, err := dispatcher.ParseEmail(t.Context(), pdf, EmailMetadata{From: seed.EmailSender, Subject: seed.SubjectContains + " (sample)", Filename: "TC_sample.pdf"})
		if err != nil || len(trades) != 1 {
			t.Fatalf("sample rows=%d err=%v", len(trades), err)
		}
		r := trades[0]
		if r.Type != "SELL" || r.Ticker != "HRTA" || r.Lot != 28 || r.Price != 1615 || r.AmountDone != 4510637 || r.Tax != 11363 || r.Date.Format("2006-01-02") != "2026-10-16" {
			t.Fatal("sample trade values differ")
		}
	}
}
