package migrations

import "database/sql"

type addTransactionTax struct{}

func (*addTransactionTax) Name() string   { return "add_transaction_tax" }
func (*addTransactionTax) SkipProd() bool { return false }
func (*addTransactionTax) Up(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE transactions ADD COLUMN IF NOT EXISTS tax DOUBLE PRECISION NOT NULL DEFAULT 0")
	return err
}

func (*addTransactionTax) Down(tx *sql.Tx) error {
	// Older installations already stored trade fees in this column.
	return nil
}
