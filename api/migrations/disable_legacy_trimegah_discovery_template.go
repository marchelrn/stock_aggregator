package migrations

import "database/sql"

const legacyTrimegahDiscoveryConfig = `{
	"schema_version": 1,
	"required_markers": ["TRIMEGAH"],
	"note": "Trimegah TC parsed by hardcoded TrimegahParser; this template exists for Gmail discovery only"
}`

type DisableLegacyTrimegahDiscoveryTemplate struct{}

func getDisableLegacyTrimegahDiscoveryTemplate() *DisableLegacyTrimegahDiscoveryTemplate {
	return &DisableLegacyTrimegahDiscoveryTemplate{}
}

func (*DisableLegacyTrimegahDiscoveryTemplate) Name() string {
	return "disable_legacy_trimegah_discovery_template"
}

func (*DisableLegacyTrimegahDiscoveryTemplate) SkipProd() bool {
	return false
}

func (*DisableLegacyTrimegahDiscoveryTemplate) Up(conn *sql.Tx) error {
	_, err := conn.Exec(`
		UPDATE pdf_parsing_templates
		SET active = FALSE, updated_at = NOW()
		WHERE key = $1 AND version = $2 AND active = TRUE
			AND config = $3::jsonb`, "trimegah", 1, legacyTrimegahDiscoveryConfig)
	return err
}

func (*DisableLegacyTrimegahDiscoveryTemplate) Down(conn *sql.Tx) error {
	return nil
}
