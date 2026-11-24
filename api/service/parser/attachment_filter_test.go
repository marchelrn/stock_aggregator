package parser

import (
	"context"
	"errors"
	"testing"

	"github.com/marchelrn/stock_api/models"
)

type attachmentTemplates []models.PDFParsingTemplate

func (rows attachmentTemplates) ListActive(context.Context) ([]models.PDFParsingTemplate, error) {
	return rows, nil
}

func TestAttachmentExclusionRespectsOtherRoutes(t *testing.T) {
	base := models.PDFParsingTemplate{
		Key: "regular", Version: 1, Active: true, BrokerName: "Test Broker",
		EmailSender: "operations@example.org",
		Config:      []byte(`{"schema_version":2,"parse_mode":"legacy","legacy_parser":"stockbit","exclude_filename_pattern":"(?i)^summary_"}`),
	}
	special := base
	special.Key = "special"
	special.Config = []byte(`{"schema_version":2,"parse_mode":"legacy","legacy_parser":"eipo","filename_pattern":"(?i)^summary_allotment_"}`)
	d := NewBrokerDispatcherParser(NewMetadataParser(attachmentTemplates{base, special}), nil, nil, nil)
	d.fallback = true
	meta := EmailMetadata{From: base.EmailSender, Filename: "SUMMARY_123.pdf"}
	if _, err := d.ParseEmail(context.Background(), nil, meta); !errors.Is(err, ErrEmailNotApplicable) {
		t.Fatalf("excluded attachment reached parser: %v", err)
	}
	meta.Filename = "summary_allotment_456.pdf"
	selected, err := d.selectTemplate(context.Background(), meta)
	if err != nil || selected == nil || selected.template.Key != special.Key {
		t.Fatalf("expected specific template, got %v, %v", selected, err)
	}
	meta.From = "other@example.org"
	if selected, err := d.selectTemplate(context.Background(), meta); selected != nil || err != nil {
		t.Fatalf("another sender inherited exclusion: %v, %v", selected, err)
	}
}
