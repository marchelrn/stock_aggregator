package migrations

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed templates/stockbit_v1.json
var stockbitV1Config []byte

type CreatePDFParsingTemplates struct{}

func getCreatePDFParsingTemplates() *CreatePDFParsingTemplates { return &CreatePDFParsingTemplates{} }
func (*CreatePDFParsingTemplates) Name() string                { return "create_pdf_parsing_templates" }
func (*CreatePDFParsingTemplates) SkipProd() bool              { return false }

func (*CreatePDFParsingTemplates) Up(conn *sql.Tx) error {
	if !json.Valid(stockbitV1Config) {
		return fmt.Errorf("invalid embedded Stockbit v1 config")
	}
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS pdf_parsing_templates (
			id BIGSERIAL PRIMARY KEY,
			key TEXT NOT NULL,
			broker_name TEXT NOT NULL,
			version INTEGER NOT NULL CHECK (version > 0),
			active BOOLEAN NOT NULL DEFAULT FALSE,
			config JSONB NOT NULL CHECK (jsonb_typeof(config) = 'object'),
			email_sender TEXT NOT NULL DEFAULT '',
			subject_contains TEXT NOT NULL DEFAULT '',
			filename_suffix TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT idx_pdf_template_key_version UNIQUE (key, version)
		)`)
	if err != nil {
		return err
	}
	// Existing user edits, activation choices and selectors are never overwritten.
	_, err = conn.Exec(`
		INSERT INTO pdf_parsing_templates (key, broker_name, version, active, config)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (key, version) DO NOTHING`,
		"stockbit", "Stockbit Sekuritas", 1, true, string(stockbitV1Config))
	return err
}

func (*CreatePDFParsingTemplates) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`DROP TABLE IF EXISTS pdf_parsing_templates`)
	return err
}
