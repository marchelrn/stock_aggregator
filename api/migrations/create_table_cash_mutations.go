package migrations

import (
	"database/sql"
	"log"
)

type CreateTableCashMutations struct{}

func getCreateTableCashMutations() migration {
	return &CreateTableCashMutations{}
}

func (m *CreateTableCashMutations) Name() string {
	return "create_cash_mutations_table"
}

func (m *CreateTableCashMutations) Up(conn *sql.Tx) error {
	log.Println("Running migration: create_cash_mutations_table")
	_, err := conn.Exec(`
	CREATE TABLE IF NOT EXISTS cash_mutations (
		id SERIAL PRIMARY KEY,
		user_id INT NOT NULL DEFAULT 0,
		broker_id INT NOT NULL,
		broker_name VARCHAR(255) NOT NULL,
		type VARCHAR(50) NOT NULL,
		amount FLOAT NOT NULL,
		date TIMESTAMP NOT NULL DEFAULT NOW(),
		description TEXT DEFAULT '',
		ref_no VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_cash_mutations_user_id ON cash_mutations(user_id);
	CREATE INDEX IF NOT EXISTS idx_cash_mutations_broker_id ON cash_mutations(broker_id);
	CREATE INDEX IF NOT EXISTS idx_cash_mutations_user_date ON cash_mutations(user_id, date);
	`)
	if err != nil {
		log.Printf("Error creating cash_mutations table: %v", err)
		return err
	}
	return nil
}

func (m *CreateTableCashMutations) Down(conn *sql.Tx) error {
	log.Println("Rolling back migration: create_cash_mutations_table")
	_, err := conn.Exec(`DROP TABLE IF EXISTS cash_mutations;`)
	return err
}

func (m *CreateTableCashMutations) SkipProd() bool {
	return false
}
