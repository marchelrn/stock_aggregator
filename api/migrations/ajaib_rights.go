package migrations

import (
	"database/sql"
	_ "embed"
	"encoding/json"

	"github.com/marchelrn/stock_api/models"
)

//go:embed templates/ajaib_trade.json
var ajaibTemplate []byte

type ajaibAndRights struct{}

func (*ajaibAndRights) Name() string   { return "ajaib_trade_and_stockbit_rights" }
func (*ajaibAndRights) SkipProd() bool { return false }
func (*ajaibAndRights) Up(tx *sql.Tx) error {
	var s models.PDFParsingTemplate
	if err := json.Unmarshal(ajaibTemplate, &s); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO pdf_parsing_templates(key,broker_name,version,active,config,email_sender,subject_contains,filename_suffix) VALUES($1,$2,$3,true,$4::jsonb,$5,$6,$7)`, s.Key, s.BrokerName, s.Version, string(s.Config), s.EmailSender, s.SubjectContains, s.FilenameSuffix)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE right_exercises (
        id bigserial PRIMARY KEY,
        user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        broker_id integer NOT NULL REFERENCES brokers(id) ON DELETE CASCADE,
        date date NOT NULL,
        ticker text NOT NULL,
        amount numeric(20,2) NOT NULL CHECK (amount > 0),
        credit_ref text NOT NULL,
        debit_ref text NOT NULL,
        message_id text NOT NULL,
        filename text NOT NULL,
        UNIQUE (user_id, broker_id, date, credit_ref),
        UNIQUE (user_id, broker_id, date, debit_ref)
    )`)
	return err
}
func (*ajaibAndRights) Down(tx *sql.Tx) error {
	_, err := tx.Exec(`DELETE FROM pdf_parsing_templates WHERE key='ajaib' AND version=1; DROP TABLE right_exercises`)
	return err
}
