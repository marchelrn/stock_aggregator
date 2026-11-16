package service

import (
	"context"
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
			},
			{
				ID:              2,
				Key:             "trimegah",
				BrokerName:      "Trimegah Sekuritas",
				Version:         1,
				Active:          true,
				EmailSender:     "trim.operation@trimegah.com",
				SubjectContains: "TC",
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
		`subject:("trade confirmation" OR "konfirmasi transaksi" OR "contract note")`,
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
