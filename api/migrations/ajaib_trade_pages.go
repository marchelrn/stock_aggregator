package migrations

import "database/sql"

type ajaibTradePages struct{}

func (*ajaibTradePages) Name() string   { return "ajaib_trade_confirmation_pages" }
func (*ajaibTradePages) SkipProd() bool { return false }

func (*ajaibTradePages) Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
		WITH added AS (
			INSERT INTO pdf_parsing_templates
				(key, broker_name, version, active, config, email_sender, subject_contains, filename_suffix)
			SELECT key, broker_name, 4, true,
				jsonb_set(
					jsonb_set(config, '{trade,page_net_pattern}', to_jsonb($1::text)),
					'{table,row_pattern}', to_jsonb(replace(config#>>'{table,row_pattern}', $2, $3))
				),
				email_sender, subject_contains, filename_suffix
			FROM pdf_parsing_templates
			WHERE key = 'ajaib' AND version = 3 AND active
			ON CONFLICT (key, version) DO NOTHING
			RETURNING key
		)
		UPDATE pdf_parsing_templates SET active = false, updated_at = NOW()
		WHERE key IN (SELECT key FROM added) AND version = 3`,
		`(?im)^[ \t]*Total Keseluruhan[ \t]*:[ \t]*(\([0-9,.]+\)|[0-9,.]+)[ \t]*\r?$`,
		`([A-Z0-9]{4}(?:-[A-Z0-9]+)?)`, `([A-Z0-9]{4}(?:-[A-Z0-9]+)?)\*?`)
	return err
}

func (*ajaibTradePages) Down(tx *sql.Tx) error {
	_, err := tx.Exec(`
		WITH removed AS (
			DELETE FROM pdf_parsing_templates WHERE key = 'ajaib' AND version = 4 RETURNING key
		)
		UPDATE pdf_parsing_templates SET active = true, updated_at = NOW()
		WHERE key IN (SELECT key FROM removed) AND version = 3`)
	return err
}
