package migrations

import "database/sql"

type verifyGmailCacheRows struct{}

func (*verifyGmailCacheRows) Name() string   { return "verify_gmail_cache_rows" }
func (*verifyGmailCacheRows) SkipProd() bool { return false }
func (*verifyGmailCacheRows) Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE processed_gmail_messages ADD COLUMN imported_ids jsonb;
		ALTER TABLE trade_reviews ADD COLUMN imported_ids jsonb;
	`)
	return err
}
func (*verifyGmailCacheRows) Down(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE trade_reviews DROP COLUMN imported_ids;
		ALTER TABLE processed_gmail_messages DROP COLUMN imported_ids;
	`)
	return err
}
