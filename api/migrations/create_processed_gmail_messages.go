package migrations

import (
	"database/sql"
)

type CreateProcessedGmailMessagesTable struct{}

func getCreateProcessedGmailMessagesTable() *CreateProcessedGmailMessagesTable {
	return &CreateProcessedGmailMessagesTable{}
}

func (*CreateProcessedGmailMessagesTable) Name() string {
	return "create_processed_gmail_messages_table"
}

func (*CreateProcessedGmailMessagesTable) SkipProd() bool {
	return false
}

func (*CreateProcessedGmailMessagesTable) Up(conn *sql.Tx) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS processed_gmail_messages (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			message_id VARCHAR(255) NOT NULL,
			message_type VARCHAR(50) NOT NULL,
			processed_at TIMESTAMP NOT NULL DEFAULT NOW(),
			trade_count INTEGER NOT NULL DEFAULT 0,
			CONSTRAINT fk_processed_gmail_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);

		CREATE UNIQUE INDEX IF NOT EXISTS idx_gmail_cache_user_msgtype
			ON processed_gmail_messages(user_id, message_id, message_type);

		CREATE INDEX IF NOT EXISTS idx_gmail_cache_processed_at
			ON processed_gmail_messages(processed_at);
	`)
	return err
}

func (*CreateProcessedGmailMessagesTable) Down(conn *sql.Tx) error {
	_, err := conn.Exec(`DROP TABLE IF EXISTS processed_gmail_messages CASCADE;`)
	return err
}
