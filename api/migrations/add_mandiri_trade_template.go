package migrations

import (
	"database/sql"
	_ "embed"
	"encoding/json"

	"github.com/marchelrn/stock_api/models"
)

//go:embed templates/mandiri_trade_v2.json
var mandiriTradeTemplate []byte

type AddMandiriTradeTemplate struct{}

func (*AddMandiriTradeTemplate) Name() string   { return "add_mandiri_trade_template" }
func (*AddMandiriTradeTemplate) SkipProd() bool { return false }
func (*AddMandiriTradeTemplate) Up(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE brokers ADD COLUMN IF NOT EXISTS pdf_password_encrypted TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	var seed models.PDFParsingTemplate
	if err := json.Unmarshal(mandiriTradeTemplate, &seed); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO pdf_parsing_templates (key,broker_name,version,active,config,email_sender,subject_contains,filename_suffix)
 VALUES ($1,$2,$3,NOT EXISTS(SELECT 1 FROM pdf_parsing_templates WHERE broker_name=$2 AND active=TRUE AND COALESCE(config->>'kind','trade')='trade'),$4::jsonb,$5,$6,$7)
 ON CONFLICT(key,version) DO NOTHING`, seed.Key, seed.BrokerName, seed.Version, string(seed.Config), seed.EmailSender, seed.SubjectContains, seed.FilenameSuffix)
	return err
}
func (*AddMandiriTradeTemplate) Down(tx *sql.Tx) error {
	var seed models.PDFParsingTemplate
	if err := json.Unmarshal(mandiriTradeTemplate, &seed); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM pdf_parsing_templates WHERE key=$1 AND version=$2 AND config=$3::jsonb AND email_sender=$4 AND subject_contains=$5 AND filename_suffix=$6`, seed.Key, seed.Version, string(seed.Config), seed.EmailSender, seed.SubjectContains, seed.FilenameSuffix)
	return err
}
