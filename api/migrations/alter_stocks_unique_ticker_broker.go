package migrations

import (
	"database/sql"
	"log"
)

type AlterStocksUniqueTickerBroker struct{}

func getAlterStocksUniqueTickerBroker() migration {
	return &AlterStocksUniqueTickerBroker{}
}

func (m *AlterStocksUniqueTickerBroker) Name() string {
	return "alter_stocks_unique_ticker_broker"
}

func (m *AlterStocksUniqueTickerBroker) SkipProd() bool {
	return false
}

func (m *AlterStocksUniqueTickerBroker) Up(conn *sql.Tx) error {
	log.Println("Running migration: alter_stocks_unique_ticker_broker")

	// Drop legacy unique constraint/index on ticker (name can vary by environment)
	_, _ = conn.Exec(`ALTER TABLE stocks DROP CONSTRAINT IF EXISTS stocks_ticker_key`)
	_, _ = conn.Exec(`DROP INDEX IF EXISTS stocks_ticker_key`)

	_, err := conn.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_stocks_ticker_broker
		ON stocks (ticker, broker_id)
	`)
	return err
}

func (m *AlterStocksUniqueTickerBroker) Down(conn *sql.Tx) error {
	_, _ = conn.Exec(`DROP INDEX IF EXISTS idx_stocks_ticker_broker`)
	_, err := conn.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS stocks_ticker_key
		ON stocks (ticker)
	`)
	return err
}
