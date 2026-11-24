package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/marchelrn/stock_api/dto"
)

func TestStockbitTableLayout(t *testing.T) {
	var templates attachmentTemplates
	load := func(file string, target any) {
		t.Helper()
		b, err := os.ReadFile("../../migrations/templates/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, target); err != nil {
			t.Fatal(err)
		}
	}
	load("trades_v2.json", &templates)
	seed := templates[0]
	if seed.Key != "stockbit" {
		t.Fatal("missing Stockbit seed")
	}
	seed.Active = true
	var cfg MetadataConfig
	if err := json.Unmarshal(seed.Config, &cfg); err != nil {
		t.Fatal(err)
	}
	load("stockbit_fee_columns.json", cfg.Trade)
	var headers struct {
		Markers []string `json:"required_markers"`
		Start   string   `json:"start_pattern"`
		Date    string   `json:"date_pattern"`
	}
	load("stockbit_table_headers.json", &headers)
	cfg.RequiredMarkers, cfg.Table.StartPattern, cfg.Date.Pattern = headers.Markers, headers.Start, headers.Date
	var err error
	seed.Config, err = json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	store := attachmentTemplates{seed}
	metadata := NewMetadataParser(store)
	dispatcher := NewBrokerDispatcherParser(metadata, nil, nil, nil)
	dispatcher.fallback = false
	pdf, err := os.ReadFile("testdata/stockbit-positioned.pdf")
	if err != nil {
		t.Fatal(err)
	}
	meta := EmailMetadata{From: seed.EmailSender, Subject: seed.SubjectContains, Filename: "TC_1234567-20260317_7654321.pdf"}
	check := func(trades []dto.ParsedTrade, err error) {
		t.Helper()
		if err != nil || len(trades) != 1 {
			t.Fatalf("parsed %v, %v", trades, err)
		}
		row := trades[0]
		if row.Type != "BUY" || row.RefNo != "0098765" || row.Ticker != "MIDI" || row.Lot != 19 || row.Price != 284 || row.Tax != 809.41 || row.AmountDone != 540409.41 {
			t.Fatalf("wrong fill or fees: %+v", row)
		}
	}
	check(dispatcher.ParseEmail(t.Context(), pdf, meta))
	check(metadata.Parse(pdf))
	// Keep stream and xref offsets intact while varying the visible labels.
	variant := bytes.ReplaceAll(pdf, []byte("Trade Confirmation"), []byte("Account  Statement"))
	variant = bytes.ReplaceAll(variant, []byte("REF # Board"), []byte("ref#  Board"))
	variant = bytes.ReplaceAll(variant, []byte("Transaction Date "), []byte("Transaction Date:"))
	check(dispatcher.ParseEmail(t.Context(), variant, meta))
	unrecognized := bytes.ReplaceAll(variant, []byte("ref#  Board"), []byte("xxxx  Board"))
	unrecognized = bytes.ReplaceAll(unrecognized, []byte("0098765 RG MIDI"), []byte("               "))
	dispatcher.fallback = true
	dispatcher.stockbit = NewStockbitParser()
	if trades, err := dispatcher.ParseEmail(t.Context(), unrecognized, meta); !errors.Is(err, ErrNoStockbitTradeRows) || !strings.Contains(err.Error(), "unmatched PDF") || len(trades) != 0 {
		t.Fatalf("expected pending PDF with marker diagnostic, got %+v, %v", trades, err)
	}
	compiled, err := compileMetadata(seed)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := compiled.extractPDF(t.Context(), pdf, "layout")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := compiled.extractPDF(t.Context(), pdf, "plain")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compiled.parseGenericTrades(plain, &meta); err == nil {
		t.Fatal("fixture must require layout retry")
	}
	check(compiled.parseGenericTrades(layout, &meta))
	twoColumns := strings.NewReplacer(
		"\nV.A.T Commission", "\nName : Example Customer       V.A.T Commission",
		"\nV.A.T Levy", "\n24 hours.                     V.A.T Levy",
	).Replace(layout)
	check(compiled.parseGenericTrades(twoColumns, &meta))
	multiline := strings.ReplaceAll(strings.ReplaceAll(layout, " 518.70 0.00", "\n518.70\n0.00"), " 57.06 0.00", "\n57.06\n0.00")
	check(compiled.parseGenericTrades(multiline, &meta))
	inconsistent := strings.Replace(layout, "1,900.00", "1,800.00", 1)
	if _, err := compiled.parseGenericTrades(inconsistent, &meta); err == nil {
		t.Fatal("inconsistent lot and share count accepted")
	}
}
