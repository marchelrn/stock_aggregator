package migrations

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/marchelrn/stock_api/models"
)

//go:embed templates/cash_v2.json
var cashV2Templates []byte

type AddMetadataCashTemplates struct{}

func getAddMetadataCashTemplates() *AddMetadataCashTemplates { return &AddMetadataCashTemplates{} }
func (*AddMetadataCashTemplates) Name() string               { return "add_metadata_cash_templates" }
func (*AddMetadataCashTemplates) SkipProd() bool             { return false }

func (*AddMetadataCashTemplates) Up(tx *sql.Tx) error {
	var seeds []models.PDFParsingTemplate
	if err := json.Unmarshal(cashV2Templates, &seeds); err != nil {
		return fmt.Errorf("decode cash templates: %w", err)
	}
	for _, seed := range seeds {
		_, err := tx.Exec(`
			INSERT INTO pdf_parsing_templates (key, broker_name, version, active, config)
			VALUES ($1, $2, $3, TRUE, $4::jsonb)
			ON CONFLICT (key, version) DO NOTHING`,
			seed.Key, seed.BrokerName, seed.Version, string(seed.Config))
		if err != nil {
			return err
		}
	}
	return nil
}

func (*AddMetadataCashTemplates) Down(tx *sql.Tx) error {
	var seeds []models.PDFParsingTemplate
	if err := json.Unmarshal(cashV2Templates, &seeds); err != nil {
		return err
	}
	for _, seed := range seeds {
		// Do not delete operator-edited templates on rollback.
		if _, err := tx.Exec(`DELETE FROM pdf_parsing_templates WHERE key = $1 AND version = $2
			AND config = $3::jsonb AND broker_name = $4 AND email_sender = ''
			AND subject_contains = '' AND filename_suffix = ''`, seed.Key, seed.Version, string(seed.Config), seed.BrokerName); err != nil {
			return err
		}
	}
	return nil
}

// Early development seeds were applied before the mailbox replay finished.
// Only those exact configurations are upgraded; operator edits are preserved.
type RefineMetadataCashSubjects struct{}

func (*RefineMetadataCashSubjects) Name() string   { return "refine_metadata_cash_subjects" }
func (*RefineMetadataCashSubjects) SkipProd() bool { return false }
func (*RefineMetadataCashSubjects) Up(tx *sql.Tx) error {
	var seeds []models.PDFParsingTemplate
	if err := json.Unmarshal(cashV2Templates, &seeds); err != nil {
		return err
	}
	for _, seed := range seeds {
		current := string(seed.Config)
		previous := current
		switch seed.Key {
		case "cash.stockbit.withdraw":
			previous = strings.Replace(current, `(?i)(penarikan|tarik dana|withdrawal)`, `(?i)^penarikan dana berhasil`, 1)
		case "cash.trimegah.withdraw":
			previous = strings.Replace(current, `(?i)^(tarik dana|penarikan|withdrawal)`, `(?i)(tarik dana|penarikan|withdrawal)`, 1)
		default:
			continue
		}
		if _, err := tx.Exec(`UPDATE pdf_parsing_templates SET config = $1::jsonb, updated_at = NOW()
			WHERE key = $2 AND version = $3 AND config = $4::jsonb`,
			current, seed.Key, seed.Version, previous); err != nil {
			return err
		}
	}
	return nil
}
func (*RefineMetadataCashSubjects) Down(*sql.Tx) error { return nil }
