package migrations

import (
	"database/sql"
	"log"
)

type AlterStocksAddSector struct{}

func getAlterStocksAddSector() migration {
	return &AlterStocksAddSector{}
}

func (m *AlterStocksAddSector) Name() string {
	return "alter_stocks_add_sector"
}

func (m *AlterStocksAddSector) SkipProd() bool {
	return false
}

func (m *AlterStocksAddSector) Up(conn *sql.Tx) error {
	log.Println("Running migration: alter_stocks_add_sector")

	_, err := conn.Exec(`
		ALTER TABLE stocks
		ADD COLUMN IF NOT EXISTS sector VARCHAR(100);

		ALTER TABLE stock_prices
		ADD COLUMN IF NOT EXISTS sector VARCHAR(100);
	`)
	return err
}

func (m *AlterStocksAddSector) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`
		ALTER TABLE stocks DROP COLUMN IF EXISTS sector;
		ALTER TABLE stock_prices DROP COLUMN IF EXISTS sector;
	`)
	return err
}
