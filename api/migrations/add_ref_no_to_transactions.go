package migrations

import (
	"database/sql"
	"log"
)

type AddRefNoToTransactions struct{}

func getAddRefNoToTransactions() migration {
	return &AddRefNoToTransactions{}
}

func (m *AddRefNoToTransactions) Name() string {
	return "add_ref_no_to_transactions"
}

func (m *AddRefNoToTransactions) Up(conn *sql.Tx) error {
	log.Println("Running migration: add_ref_no_to_transactions")
	_, err := conn.Exec(`
	ALTER TABLE transactions
	ADD COLUMN IF NOT EXISTS ref_no VARCHAR(255) DEFAULT ''
	`)
	if err != nil {
		log.Printf("Error adding ref_no column to transactions table: %v", err)
		return err
	}

	// Create unique index on (user_id, ref_no) for duplicate prevention
	_, err = conn.Exec(`
	CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_user_ref_no 
	ON transactions (user_id, ref_no) 
	WHERE ref_no != ''
	`)
	if err != nil {
		log.Printf("Error creating unique index on transactions ref_no: %v", err)
		return err
	}

	log.Println("Successfully added ref_no column and unique index to transactions table")
	return nil
}

func (m *AddRefNoToTransactions) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`DROP INDEX IF EXISTS idx_transactions_user_ref_no`)
	if err != nil {
		return err
	}
	_, err = conn.Exec(`ALTER TABLE transactions DROP COLUMN IF EXISTS ref_no`)
	return err
}

func (m *AddRefNoToTransactions) SkipProd() bool {
	return false
}