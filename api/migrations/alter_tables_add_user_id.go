package migrations

import (
	"database/sql"
	"log"
)

type AlterTablesAddUserID struct{}

func getAlterTablesAddUserID() migration {
	return &AlterTablesAddUserID{}
}

func (m *AlterTablesAddUserID) Name() string {
	return "alter_tables_add_user_id"
}

func (m *AlterTablesAddUserID) SkipProd() bool {
	return false
}

func (m *AlterTablesAddUserID) Up(conn *sql.Tx) error {
	log.Println("Running migration: alter_tables_add_user_id")

	// 1. Brokers: add user_id, adjust unique constraints
	_, err := conn.Exec(`
		ALTER TABLE brokers
		ADD COLUMN IF NOT EXISTS user_id INT;

		ALTER TABLE brokers DROP CONSTRAINT IF EXISTS brokers_name_key;
		DROP INDEX IF EXISTS brokers_name_key;

		CREATE UNIQUE INDEX IF NOT EXISTS idx_brokers_user_name
		ON brokers (user_id, name);
	`)
	if err != nil {
		return err
	}

	// 2. Stocks: add user_id, adjust unique constraints
	_, err = conn.Exec(`
		ALTER TABLE stocks
		ADD COLUMN IF NOT EXISTS user_id INT;

		DROP INDEX IF EXISTS idx_stocks_ticker_broker;
		ALTER TABLE stocks DROP CONSTRAINT IF EXISTS stocks_ticker_key;
		DROP INDEX IF EXISTS stocks_ticker_key;

		CREATE UNIQUE INDEX IF NOT EXISTS idx_stocks_user_ticker_broker
		ON stocks (user_id, ticker, broker_id);
	`)
	if err != nil {
		return err
	}

	// 3. Transactions: add user_id, add index
	_, err = conn.Exec(`
		ALTER TABLE transactions
		ADD COLUMN IF NOT EXISTS user_id INT;

		CREATE INDEX IF NOT EXISTS idx_transactions_user_id
		ON transactions (user_id);
	`)
	if err != nil {
		return err
	}

	// 4. Migrate any existing records to the first user if available
	_, _ = conn.Exec(`
		DO $$
		DECLARE
			first_user_id INT;
		BEGIN
			SELECT id INTO first_user_id FROM users ORDER BY id ASC LIMIT 1;
			IF first_user_id IS NOT NULL THEN
				UPDATE brokers SET user_id = first_user_id WHERE user_id IS NULL;
				UPDATE stocks SET user_id = first_user_id WHERE user_id IS NULL;
				UPDATE transactions SET user_id = first_user_id WHERE user_id IS NULL;
			END IF;
		END $$;
	`)

	return nil
}

func (m *AlterTablesAddUserID) Down(conn *sql.Tx) error {
	_, _ = conn.Exec(`DROP INDEX IF EXISTS idx_brokers_user_name;`)
	_, _ = conn.Exec(`DROP INDEX IF EXISTS idx_stocks_user_ticker_broker;`)
	_, _ = conn.Exec(`DROP INDEX IF EXISTS idx_transactions_user_id;`)
	return nil
}
