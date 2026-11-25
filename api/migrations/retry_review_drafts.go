package migrations

import "database/sql"

type retryReviewDrafts struct{}

func (*retryReviewDrafts) Name() string   { return "retry_manual_review_drafts_on_sync" }
func (*retryReviewDrafts) SkipProd() bool { return false }
func (*retryReviewDrafts) Up(tx *sql.Tx) error {
	_, err := tx.Exec("UPDATE trade_reviews SET state='pending' WHERE state='review'")
	return err
}
func (*retryReviewDrafts) Down(tx *sql.Tx) error {
	_, err := tx.Exec("UPDATE trade_reviews SET state='review' WHERE state='pending' AND COALESCE(draft,'') <> ''")
	return err
}
