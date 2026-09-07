package migrations

import (
	"database/sql"
	"log"
)

type CreateStocksTable struct{}

func (m *CreateStocksTable) SkipProd() bool {
	return false
}

func getCreateStocksTable() *CreateStocksTable {
	return &CreateStocksTable{}
}

func (m *CreateStocksTable) Name() string {
	return "create_stocks_table"
}

func (m *CreateStocksTable) Up(conn *sql.Tx) error {
	_, err := conn.Exec(`
	CREATE TABLE IF NOT EXISTS stocks (
		id SERIAL PRIMARY KEY,
		ticker VARCHAR(10) NOT NULL UNIQUE,
		lot INT NOT NULL,
		avg_price NUMERIC(10, 2) NOT NULL,
		broker_name VARCHAR(255) NOT NULL,
		broker_id INT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)
	`)
	log.Println("Creating up migrations : stocks-table")
	return err
}

func (m *CreateStocksTable) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`DROP TABLE IF NOT EXISTS stocks`)
	if err != nil {
		return err
	}
	return err
}
