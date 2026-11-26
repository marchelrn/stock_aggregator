package migrations

import "database/sql"

type ajaibOptionalPDFTitle struct{}

func (*ajaibOptionalPDFTitle) Name() string   { return "ajaib_optional_pdf_title" }
func (*ajaibOptionalPDFTitle) SkipProd() bool { return false }

func (*ajaibOptionalPDFTitle) Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
		WITH added AS (
			INSERT INTO pdf_parsing_templates
				(key, broker_name, version, active, config, email_sender, subject_contains, filename_suffix)
			SELECT key, broker_name, 3, true,
				jsonb_set(config, '{required_markers}', (config->'required_markers') - 'Rangkuman Transaksi Saham Harian'),
				email_sender, subject_contains, filename_suffix
			FROM pdf_parsing_templates
			WHERE key = 'ajaib' AND version = 2 AND active
			  AND (config->'required_markers') ? 'Rangkuman Transaksi Saham Harian'
			ON CONFLICT (key, version) DO NOTHING
			RETURNING key
		)
		UPDATE pdf_parsing_templates SET active = false, updated_at = NOW()
		WHERE key IN (SELECT key FROM added) AND version = 2`)
	return err
}

func (*ajaibOptionalPDFTitle) Down(tx *sql.Tx) error {
	_, err := tx.Exec(`
		WITH removed AS (
			DELETE FROM pdf_parsing_templates WHERE key = 'ajaib' AND version = 3 RETURNING key
		)
		UPDATE pdf_parsing_templates SET active = true, updated_at = NOW()
		WHERE key IN (SELECT key FROM removed) AND version = 2`)
	return err
}
