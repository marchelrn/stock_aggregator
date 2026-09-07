package migrations

import (
	"database/sql"
	"log"
)

type CreateTableBroker struct{}

func (m *CreateTableBroker) SkipProd() bool {
	return false
}

func getCreateTableBroker() *CreateTableBroker {
	return &CreateTableBroker{}
}

func (m *CreateTableBroker) Name() string {
	return "create_table_broker"
}

func (m *CreateTableBroker) Up(conn *sql.Tx) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS brokers (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			cash NUMERIC(10, 2) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMP
	)`)
	log.Println("Creating up migrations : broker-table")
	return err
}

func (m *CreateTableBroker) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`DROP TABLE IF NOT EXISTS brokers`)
	if err != nil {
		return err
	}
	return err
}
