package migrations

import "database/sql"

type addQuoteTime struct{}

func (*addQuoteTime) Name() string   { return "add_stock_price_quote_time" }
func (*addQuoteTime) SkipProd() bool { return false }
func (*addQuoteTime) Up(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE stock_prices ADD COLUMN IF NOT EXISTS quoted_at TIMESTAMPTZ")
	return err
}
func (*addQuoteTime) Down(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE stock_prices DROP COLUMN IF EXISTS quoted_at")
	return err
}
