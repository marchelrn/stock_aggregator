package migrations

import "database/sql"

type ajaibTCAttachmentNames struct{}

func (*ajaibTCAttachmentNames) Name() string   { return "ajaib_tc_attachment_names" }
func (*ajaibTCAttachmentNames) SkipProd() bool { return false }

func (*ajaibTCAttachmentNames) Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
		WITH added AS (
			INSERT INTO pdf_parsing_templates
				(key, broker_name, version, active, config, email_sender, subject_contains, filename_suffix)
			SELECT key, broker_name, 5, true,
				config || jsonb_build_object('filename_pattern', $1::text, 'require_filename_match', true),
				email_sender, subject_contains, filename_suffix
			FROM pdf_parsing_templates WHERE key = 'ajaib' AND version = 4 AND active
			ON CONFLICT (key, version) DO NOTHING
			RETURNING key
		)
		UPDATE pdf_parsing_templates SET active = false, updated_at = NOW()
		WHERE key IN (SELECT key FROM added) AND version = 4`, `(?i)^tc[_-][^/\\]+\.pdf$`)
	return err
}

func (*ajaibTCAttachmentNames) Down(tx *sql.Tx) error {
	_, err := tx.Exec(`
		WITH removed AS (
			DELETE FROM pdf_parsing_templates WHERE key = 'ajaib' AND version = 5 RETURNING key
		)
		UPDATE pdf_parsing_templates SET active = true, updated_at = NOW()
		WHERE key IN (SELECT key FROM removed) AND version = 4`)
	return err
}
