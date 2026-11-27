package migrations

import "database/sql"

type portfolioPerformance struct{}

func (*portfolioPerformance) Name() string   { return "portfolio_performance" }
func (*portfolioPerformance) SkipProd() bool { return false }

func (*portfolioPerformance) Up(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE portfolio_performance (
		user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		session_date DATE NOT NULL,
		broker_id INT NOT NULL,
		broker_name TEXT NOT NULL,
		total_value NUMERIC,
		unpriced_holdings INT NOT NULL,
		captured_at TIMESTAMPTZ NOT NULL,
		PRIMARY KEY (user_id, session_date, broker_id)
	)`)
	return err
}

func (*portfolioPerformance) Down(tx *sql.Tx) error {
	_, err := tx.Exec("DROP TABLE portfolio_performance")
	return err
}

type portfolioPriceObservation struct{}

func (*portfolioPriceObservation) Name() string   { return "portfolio_price_observation" }
func (*portfolioPriceObservation) SkipProd() bool { return false }

func (*portfolioPriceObservation) Up(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE stock_prices ADD COLUMN IF NOT EXISTS observed_at TIMESTAMPTZ")
	return err
}

func (*portfolioPriceObservation) Down(tx *sql.Tx) error {
	_, err := tx.Exec("ALTER TABLE stock_prices DROP COLUMN IF EXISTS observed_at")
	return err
}
