package migrations

import "database/sql"

// UpgradePDFTemplatesV2 upgrades the two seeded templates to schema version 2.
// Routing moves into the database: both brokers still use their built-in
// extractors (parse_mode legacy), but the dispatcher now reads which extractor
// applies from the template instead of sniffing email strings. Trimegah is
// activated again as the discovery source, replacing its hardcoded selectors.
type UpgradePDFTemplatesV2 struct{}

func getUpgradePDFTemplatesV2() *UpgradePDFTemplatesV2 { return &UpgradePDFTemplatesV2{} }
func (*UpgradePDFTemplatesV2) Name() string            { return "upgrade_pdf_templates_v2" }
func (*UpgradePDFTemplatesV2) SkipProd() bool          { return false }

const trimegahV2Config = `{
	"schema_version": 2,
	"document": "pdf",
	"kind": "trade",
	"parse_mode": "legacy",
	"legacy_parser": "trimegah",
	"required_markers": ["TRIMEGAH"]
}`

func (*UpgradePDFTemplatesV2) Up(conn *sql.Tx) error {
	// Preserve every Stockbit pattern; only add the routing fields.
	if _, err := conn.Exec(`
		UPDATE pdf_parsing_templates
		SET config = config || '{
			"schema_version": 2,
			"document": "pdf",
			"kind": "trade",
			"parse_mode": "legacy",
			"legacy_parser": "stockbit"
		}'::jsonb, updated_at = NOW()
		WHERE key = 'stockbit' AND version = 1
			AND config = $1::jsonb`, string(stockbitV1Config)); err != nil {
		return err
	}

	// The seeded Trimegah row was discovery-only and got disabled. Replace its
	// minimal config with a valid version 2 routing row and enable it, so its
	// selectors no longer need to be hardcoded in the dispatcher.
	_, err := conn.Exec(`
		UPDATE pdf_parsing_templates
		SET config = $1::jsonb,
			active = TRUE,
			email_sender = 'trim.operation@trimegah.com',
			subject_contains = '',
			filename_suffix = '',
			updated_at = NOW()
		WHERE key = 'trimegah' AND version = 1
			AND config = $2::jsonb
			AND email_sender = 'trim.operation@trimegah.com'
			AND subject_contains = 'TC'
			AND filename_suffix = ''`, trimegahV2Config, legacyTrimegahDiscoveryConfig)
	return err
}

func (*UpgradePDFTemplatesV2) Down(conn *sql.Tx) error {
	if _, err := conn.Exec(`
		UPDATE pdf_parsing_templates
		SET config = (config - 'document' - 'kind' - 'parse_mode' - 'legacy_parser')
			|| '{"schema_version": 1}'::jsonb, updated_at = NOW()
		WHERE key = 'stockbit' AND version = 1
			AND config = $1::jsonb || '{
				"schema_version": 2, "document": "pdf", "kind": "trade",
				"parse_mode": "legacy", "legacy_parser": "stockbit"
			}'::jsonb`, string(stockbitV1Config)); err != nil {
		return err
	}
	_, err := conn.Exec(`
		UPDATE pdf_parsing_templates
		SET config = $1::jsonb, active = FALSE, subject_contains = 'TC', updated_at = NOW()
		WHERE key = 'trimegah' AND version = 1
			AND config = $2::jsonb
			AND email_sender = 'trim.operation@trimegah.com'
			AND subject_contains = ''
			AND filename_suffix = ''`, legacyTrimegahDiscoveryConfig, trimegahV2Config)
	return err
}
