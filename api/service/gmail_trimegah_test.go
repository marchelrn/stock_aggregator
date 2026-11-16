package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/service/parser"
)

type testTemplateStore struct {
	templates []models.PDFParsingTemplate
}

func (s *testTemplateStore) ListActive(ctx context.Context) ([]models.PDFParsingTemplate, error) {
	return s.templates, nil
}

// TestGmailQueryIncludesTrimegah verifies that when Trimegah template is active,
// the Gmail sync query includes trim.operation@trimegah.com as a sender filter.
func TestGmailQueryIncludesTrimegah(t *testing.T) {
	config, err := json.Marshal(parser.MetadataConfig{
		SchemaVersion:   1,
		RequiredMarkers: []string{"SYNTHETIC"},
		SharesPerLot:    100,
		Date:            parser.MetadataDate{Pattern: `Date: (\d{4}-\d{2}-\d{2})`, Group: 1, Layout: "2006-01-02"},
		Table: parser.MetadataTable{
			StartPattern: "BEGIN", EndPattern: "END",
			RowPattern: `([A-Z]+) (\d+) (\d+) ([0-9.]+) ([0-9.]+) ([0-9.]+)`,
			Fields:     map[string]int{"ticker": 1, "lot": 2, "quantity": 3, "price": 4, "buy_amount": 5, "sell_amount": 6},
		},
		Numbers: parser.MetadataNumbers{DecimalSeparator: ".", ThousandsSeparator: ",", GroupingSize: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	store := &testTemplateStore{
		templates: []models.PDFParsingTemplate{
			{
				ID:              1,
				Key:             "stockbit",
				BrokerName:      "Stockbit Sekuritas",
				Version:         1,
				Active:          true,
				EmailSender:     "no-reply@stockbit.com",
				SubjectContains: "Trade Confirmation",
				Config:          config,
			},
			{
				ID:              2,
				Key:             "trimegah",
				BrokerName:      "Trimegah Sekuritas",
				Version:         1,
				Active:          true,
				EmailSender:     "trim.operation@trimegah.com",
				SubjectContains: "TC",
				Config:          config,
			},
		},
	}

	metadataParser := parser.NewMetadataParser(store)
	stockbitParser := parser.NewStockbitParser()
	mandiriParser := parser.NewMandiriParser()
	trimegahParser := parser.NewTrimegahParser()
	dispatcher := parser.NewBrokerDispatcherParser(metadataParser, stockbitParser, mandiriParser, trimegahParser)

	service := &GmailServiceImpl{
		TradeParser: dispatcher,
	}

	query, err := service.tradeConfirmationQuery(context.Background(), nil)
	if err != nil {
		t.Fatalf("tradeConfirmationQuery failed: %v", err)
	}

	// Query harus include kedua sender
	expectedFragments := []string{
		`subject:("trade confirmation" OR "konfirmasi transaksi" OR "contract note" OR "tc")`,
		"has:attachment",
		"filename:pdf",
		`from:"no-reply@stockbit.com"`,
		`from:"trim.operation@trimegah.com"`,
	}

	for _, fragment := range expectedFragments {
		if !contains(query, fragment) {
			t.Errorf("query missing fragment %q\nGot: %s", fragment, query)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
