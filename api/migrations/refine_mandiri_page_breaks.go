package migrations

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/marchelrn/stock_api/models"
)

type RefineMandiriPageBreaks struct{}

func (*RefineMandiriPageBreaks) Name() string   { return "refine_mandiri_page_breaks" }
func (*RefineMandiriPageBreaks) SkipProd() bool { return false }
func mandiriPageBreakConfigs() (current, previous string, err error) {
	var seed models.PDFParsingTemplate
	if err = json.Unmarshal(mandiriTradeTemplate, &seed); err != nil {
		return
	}
	current = string(seed.Config)
	var cfg map[string]json.RawMessage
	if err = json.Unmarshal(seed.Config, &cfg); err != nil {
		return
	}
	var trade map[string]json.RawMessage
	if err = json.Unmarshal(cfg["trade"], &trade); err != nil {
		return
	}
	delete(trade, "ignore_patterns")
	cfg["trade"], err = json.Marshal(trade)
	if err != nil {
		return
	}
	var raw []byte
	raw, err = json.Marshal(cfg)
	previous = string(raw)
	return
}
func (*RefineMandiriPageBreaks) Up(tx *sql.Tx) error {
	current, previous, err := mandiriPageBreakConfigs()
	if err != nil {
		return err
	}
	initial := strings.Replace(previous, "(?m)Sales Tax", "(?m)^[ \\\\t]*Sales Tax", 1)
	_, err = tx.Exec(`UPDATE pdf_parsing_templates SET config=$1::jsonb,updated_at=NOW()
 WHERE key='mandiri' AND version=1 AND config IN ($2::jsonb, $3::jsonb)`, current, previous, initial)
	return err
}
func (*RefineMandiriPageBreaks) Down(tx *sql.Tx) error {
	current, previous, err := mandiriPageBreakConfigs()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE pdf_parsing_templates SET config=$1::jsonb,updated_at=NOW()
 WHERE key='mandiri' AND version=1 AND config=$2::jsonb`, previous, current)
	return err
}
