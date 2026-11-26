package migrations

import "database/sql"

type ajaibSenderDomain struct{}

func (*ajaibSenderDomain) Name() string   { return "ajaib_trade_sender_domain" }
func (*ajaibSenderDomain) SkipProd() bool { return false }

func (*ajaibSenderDomain) Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
		WITH added AS (
			INSERT INTO pdf_parsing_templates
				(key, broker_name, version, active, config, email_sender, subject_contains, filename_suffix)
			SELECT key, broker_name, 2, true,
				(config - 'subject_pattern') || jsonb_build_object(
					'sender_domain', 'ajaib.co.id', 'filename_pattern', $1::text
				),
				'', '', filename_suffix
			FROM pdf_parsing_templates
			WHERE key = 'ajaib' AND version = 1 AND active
			ON CONFLICT (key, version) DO NOTHING
			RETURNING key
		)
		UPDATE pdf_parsing_templates SET active = false, updated_at = NOW()
		WHERE key IN (SELECT key FROM added) AND version = 1`,
		`(?i)\.pdf$`)
	return err
}

func (*ajaibSenderDomain) Down(tx *sql.Tx) error {
	_, err := tx.Exec(`
		WITH removed AS (
			DELETE FROM pdf_parsing_templates WHERE key = 'ajaib' AND version = 2 RETURNING key
		)
		UPDATE pdf_parsing_templates SET active = true, updated_at = NOW()
		WHERE key IN (SELECT key FROM removed) AND version = 1`)
	return err
}
