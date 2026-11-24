package migrations

import "database/sql"

type addCashSyncCheckpoint struct{}

func (*addCashSyncCheckpoint) Name() string   { return "add_broker_cash_sync_checkpoint" }
func (*addCashSyncCheckpoint) SkipProd() bool { return false }
func (*addCashSyncCheckpoint) Up(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE brokers ADD COLUMN last_cash_sync_at TIMESTAMPTZ")
	return err
}
func (*addCashSyncCheckpoint) Down(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE brokers DROP COLUMN last_cash_sync_at")
	return err
}
