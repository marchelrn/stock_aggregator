package migrations

import "database/sql"

type addTradeEmailSentAt struct{}

func (*addTradeEmailSentAt) Name() string   { return "add_trade_email_sent_at" }
func (*addTradeEmailSentAt) SkipProd() bool { return false }

func (*addTradeEmailSentAt) Up(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE transactions ADD COLUMN email_sent_at TIMESTAMPTZ")
	return err
}

func (*addTradeEmailSentAt) Down(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE transactions DROP COLUMN email_sent_at")
	return err
}
