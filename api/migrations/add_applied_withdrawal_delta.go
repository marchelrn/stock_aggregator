package migrations

import "database/sql"

type AddAppliedWithdrawalDelta struct{}

func (*AddAppliedWithdrawalDelta) Name() string   { return "add_applied_withdrawal_delta" }
func (*AddAppliedWithdrawalDelta) SkipProd() bool { return false }
func (*AddAppliedWithdrawalDelta) Up(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE cash_mutations ADD COLUMN applied_delta DOUBLE PRECISION")
	return err
}
func (*AddAppliedWithdrawalDelta) Down(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE cash_mutations DROP COLUMN applied_delta")
	return err
}
