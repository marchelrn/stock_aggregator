package migrations

import "database/sql"

type tradeReviews struct{}

func (*tradeReviews) Name() string   { return "gmail_trade_reviews" }
func (*tradeReviews) SkipProd() bool { return false }
func (*tradeReviews) Up(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE trade_reviews (
		id bigserial PRIMARY KEY, user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		broker_id integer NOT NULL REFERENCES brokers(id) ON DELETE CASCADE, broker_name text NOT NULL,
		message_id text NOT NULL, part_id text NOT NULL, digest text NOT NULL, filename text NOT NULL,
		state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','review','saved')),
		draft text NOT NULL DEFAULT '', consented_at timestamptz, consent_source text, extraction_lease text, UNIQUE(user_id, broker_id, digest));
		ALTER TABLE transactions ADD COLUMN reviewed_fee boolean NOT NULL DEFAULT false`)
	return err
}
func (*tradeReviews) Down(tx *sql.Tx) error {
	_, err := tx.Exec("DROP TABLE trade_reviews; ALTER TABLE transactions DROP COLUMN reviewed_fee")
	return err
}
