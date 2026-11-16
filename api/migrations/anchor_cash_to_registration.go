package migrations

import "database/sql"

// Preserve the unverified stored balance at cutover, rather than replaying
// history or pretending historical imports can tell us the user's real cash.
const anchorCashToRegistrationSQL = `
WITH cutoffs AS (
	SELECT b.id, b.user_id,
		CASE WHEN b.cash_base_as_of > u.created_at
			THEN b.cash_base_as_of ELSE u.created_at END AS cutoff
	FROM brokers b JOIN users u ON u.id = b.user_id
	WHERE u.created_at IS NOT NULL
), totals AS (
	SELECT c.id, c.cutoff,
		COALESCE((SELECT SUM(
			CASE WHEN t.type = 'BUY' THEN -(t.lot * 100.0 * t.price) ELSE t.lot * 100.0 * t.price END
			- CASE WHEN COALESCE(t.tax, 0) <> 0 THEN t.tax
				WHEN t.type = 'BUY' THEN t.lot * 100.0 * t.price * 0.0015
				ELSE t.lot * 100.0 * t.price * 0.0025 END
		) FROM transactions t
		WHERE t.user_id = c.user_id AND t.broker_id = c.id AND t.date > c.cutoff), 0)
		+ COALESCE((SELECT SUM(CASE
			WHEN m.type IN ('DEPOSIT', 'DIVIDEND') THEN m.amount
			WHEN m.type = 'WITHDRAW' THEN -m.amount ELSE 0 END)
		FROM cash_mutations m
		WHERE m.user_id = c.user_id AND m.broker_id = c.id AND m.date > c.cutoff), 0) AS delta
	FROM cutoffs c
)
UPDATE brokers
SET cash_base = brokers.cash - totals.delta, cash_base_as_of = totals.cutoff
FROM totals WHERE brokers.id = totals.id
`

type AnchorCashToRegistration struct{}

func getAnchorCashToRegistration() migration     { return &AnchorCashToRegistration{} }
func (*AnchorCashToRegistration) Name() string   { return "anchor_cash_to_registration" }
func (*AnchorCashToRegistration) SkipProd() bool { return false }

func (*AnchorCashToRegistration) Up(conn *sql.Tx) error {
	_, err := conn.Exec(anchorCashToRegistrationSQL)
	return err
}

// Restoring old eligibility would make new events unsafe; balance metadata
// cannot be rolled back accurately once cash activity has continued.
func (*AnchorCashToRegistration) Down(conn *sql.Tx) error { return nil }
