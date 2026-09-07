package migrations

import (
	"database/sql"
	"log"
)

type CreateTableUsers struct{}

func getCreateTableUsers() *CreateTableUsers {
	return &CreateTableUsers{}
}

func (m *CreateTableUsers) SkipProd() bool {
	return false
}

func (m *CreateTableUsers) Name() string {
	return "create_table_users"
}

func (m *CreateTableUsers) Up(conn *sql.Tx) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id				SERIAL PRIMARY KEY,
			email			VARCHAR(255) NOT NULL UNIQUE,
			name			VARCHAR(255) NOT NULL,
			password		VARCHAR(255) NULL,	
			auth_provider		VARCHAR(50) NOT NULL DEFAULT 'local',
			provider_id			VARCHAR(255),
			created_at			TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at			TIMESTAMP NOT NULL DEFAULT NOW(),
			deleted_at			TIMESTAMP
		);

		CREATE UNIQUE INDEX idx_users_email ON users(email);
		CREATE UNIQUE INDEX idx_users_auth ON users(auth_provider, provider_id);
	`)
	log.Println("Creating up migrations : users-table")
	return err
}

func (m *CreateTableUsers) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`
		DROP TABLE IF EXISTS users;
	`)
	return err
}
