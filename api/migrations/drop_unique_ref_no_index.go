package migrations

import (
	"database/sql"
	"log"
)

// DropUniqueRefNoIndex replaces the unique partial index on
// (user_id, ref_no) with a plain lookup index. A single trade confirmation
// legitimately contains several fills that share the same Ref# (partial
// executions at different prices), so the unique index aborted batches as
// soon as such a confirmation was stored. Duplicate prevention is handled in
// the application by row identity (ticker/type/lot/price/date/broker).
type DropUniqueRefNoIndex struct{}

func getDropUniqueRefNoIndex() migration {
	return &DropUniqueRefNoIndex{}
}

func (m *DropUniqueRefNoIndex) Name() string {
	return "drop_unique_ref_no_index"
}

func (m *DropUniqueRefNoIndex) Up(conn *sql.Tx) error {
	log.Println("Running migration: drop_unique_ref_no_index")
	if _, err := conn.Exec(`DROP INDEX IF EXISTS idx_transactions_user_ref_no`); err != nil {
		log.Printf("Error dropping unique index on transactions ref_no: %v", err)
		return err
	}
	// Keep a non-unique index for lookups by RefNo.
	_, err := conn.Exec(`
	CREATE INDEX IF NOT EXISTS idx_transactions_user_ref_no
	ON transactions (user_id, ref_no)
	WHERE ref_no != ''
	`)
	if err != nil {
		log.Printf("Error creating plain index on transactions ref_no: %v", err)
		return err
	}
	log.Println("Successfully replaced unique ref_no index with a plain index")
	return nil
}

func (m *DropUniqueRefNoIndex) Down(conn *sql.Tx) error {
	if _, err := conn.Exec(`DROP INDEX IF EXISTS idx_transactions_user_ref_no`); err != nil {
		return err
	}
	_, err := conn.Exec(`
	CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_user_ref_no
	ON transactions (user_id, ref_no)
	WHERE ref_no != ''
	`)
	return err
}

func (m *DropUniqueRefNoIndex) SkipProd() bool {
	return false
}
