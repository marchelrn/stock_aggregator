package migrations

import (
	"database/sql"
	"log"
)

type CreateStockPricesTable struct{}

func (m *CreateStockPricesTable) SkipProd() bool {
	return false
}

func getCreateStockPricesTable() *CreateStockPricesTable {
	return &CreateStockPricesTable{}
}

func (m *CreateStockPricesTable) Name() string {
	return "create_stock_prices_table"
}

func (m *CreateStockPricesTable) Up(conn *sql.Tx) error {
	_, err := conn.Exec(`
	CREATE TABLE IF NOT EXISTS stock_prices (
		id SERIAL PRIMARY KEY,
		ticker VARCHAR(10) NOT NULL UNIQUE,
		price NUMERIC(15, 2) NOT NULL,
		previous_close NUMERIC(15, 2) NOT NULL,
		change NUMERIC(15, 2) NOT NULL,
		change_percent NUMERIC(15, 2) NOT NULL,
		currency VARCHAR(10) NOT NULL,
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)
	`)
	log.Println("Creating up migrations : stock_prices-table")
	return err
}

func (m *CreateStockPricesTable) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`DROP TABLE IF NOT EXISTS stock_prices`)
	return err
}
