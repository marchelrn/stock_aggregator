package migrations

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/marchelrn/stock_api/models"
)

type RefineStockbitDepositSubject struct{}

func (*RefineStockbitDepositSubject) Name() string   { return "refine_stockbit_deposit_subject" }
func (*RefineStockbitDepositSubject) SkipProd() bool { return false }

func stockbitDepositConfigs() (current, previous string, err error) {
	var seeds []models.PDFParsingTemplate
	if err = json.Unmarshal(cashV2Templates, &seeds); err != nil {
		return
	}
	for _, seed := range seeds {
		if seed.Key != "cash.stockbit.deposit" {
			continue
		}
		current = string(seed.Config)
		var config map[string]json.RawMessage
		if err = json.Unmarshal(seed.Config, &config); err != nil {
			return
		}
		config["subject_pattern"] = json.RawMessage(`"(?i)(deposit|dana masuk)"`)
		var data []byte
		data, err = json.Marshal(config)
		previous = string(data)
		return
	}
	err = errors.New("Stockbit deposit seed not found")
	return
}

func (*RefineStockbitDepositSubject) Up(tx *sql.Tx) error {
	current, previous, err := stockbitDepositConfigs()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE pdf_parsing_templates SET config = $1::jsonb, updated_at = NOW()
		WHERE key = 'cash.stockbit.deposit' AND version = 1 AND config = $2::jsonb`, current, previous)
	return err
}

func (*RefineStockbitDepositSubject) Down(tx *sql.Tx) error {
	current, previous, err := stockbitDepositConfigs()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE pdf_parsing_templates SET config = $1::jsonb, updated_at = NOW()
		WHERE key = 'cash.stockbit.deposit' AND version = 1 AND config = $2::jsonb`, previous, current)
	return err
}
