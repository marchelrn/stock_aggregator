package migrations

import (
	"database/sql"
	"log"
)

type CreateTableTransaction struct{}

func getCreateTableTransaction() migration {
	return &CreateTableTransaction{}
}

func (m *CreateTableTransaction) Name() string {
	return "create_transaction_table"
}

func (m *CreateTableTransaction) Up(conn *sql.Tx) error {
	log.Println("Running migration: create_transaction_table")
	_, err := conn.Exec(`
	CREATE TABLE IF NOT EXISTS transactions (
		id SERIAL PRIMARY KEY,
		ticker VARCHAR(255) NOT NULL,
		price FLOAT NOT NULL,
		lot INT NOT NULL,
		broker_id INT NOT NULL,
		broker_name VARCHAR(255) NOT NULL,
		type VARCHAR(255) NOT NULL,
		date TIMESTAMP NOT NULL DEFAULT NOW(),
		stock_id INT NOT NULL
	)`)
	if err != nil {
		log.Printf("Error creating transactions table: %v", err)
		return err
	}
	_, err = conn.Exec(`
	ALTER TABLE transactions
	ADD COLUMN IF NOT EXISTS stock_id INT NOT NULL
	`)
	if err != nil {
		log.Printf("Error adding stock_id column to transactions table: %v", err)
		return err
	}

	_, err = conn.Exec(`
	ALTER TABLE transactions
	ADD CONSTRAINT fk_stock_id FOREIGN KEY (stock_id) REFERENCES stocks(id) ON DELETE CASCADE
	`)
	if err != nil {
		log.Printf("Error adding foreign key constraint to transactions table: %v", err)
		return err
	}

	_, err = conn.Exec(`
	ALTER TABLE transactions
	ADD COLUMN IF NOT EXISTS amount_done float NOT NULL
	`)
	if err != nil {
		log.Printf("Error adding amount_done column to transactions table: %v", err)
		return err
	}

	log.Println("Successfully created transactions table")
	return nil
}

func (m *CreateTableTransaction) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`DROP TABLE IF EXISTS transactions`)
	if err != nil {
		return err
	}
	return err
}

func (m *CreateTableTransaction) SkipProd() bool {
	return false
}
