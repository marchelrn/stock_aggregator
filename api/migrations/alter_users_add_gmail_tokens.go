package migrations

import (
	"database/sql"
	"log"
)

type AlterUsersAddGmailTokens struct{}

func getAlterUsersAddGmailTokens() *AlterUsersAddGmailTokens {
	return &AlterUsersAddGmailTokens{}
}

func (m *AlterUsersAddGmailTokens) SkipProd() bool {
	return false
}

func (m *AlterUsersAddGmailTokens) Name() string {
	return "alter_users_add_gmail_tokens"
}

func (m *AlterUsersAddGmailTokens) Up(conn *sql.Tx) error {
	_, err := conn.Exec(`
		ALTER TABLE users 
			ADD COLUMN IF NOT EXISTS gmail_access_token TEXT,
			ADD COLUMN IF NOT EXISTS gmail_refresh_token TEXT,
			ADD COLUMN IF NOT EXISTS gmail_token_expiry TIMESTAMP,
			ADD COLUMN IF NOT EXISTS gmail_sync_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN IF NOT EXISTS last_gmail_sync_at TIMESTAMP
	`)
	log.Println("Creating up migrations : alter_users_add_gmail_tokens")
	return err
}

func (m *AlterUsersAddGmailTokens) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`
		ALTER TABLE users
			DROP COLUMN IF EXISTS gmail_access_token,
			DROP COLUMN IF EXISTS gmail_refresh_token,
			DROP COLUMN IF EXISTS gmail_token_expiry,
			DROP COLUMN IF EXISTS gmail_sync_enabled,
			DROP COLUMN IF EXISTS last_gmail_sync_at;
	`)
	return err
}
