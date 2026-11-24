package migrations

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"

	"github.com/marchelrn/stock_api/models"
)

//go:embed templates/stockbit_table_layout.json
var stockbitLayoutPatch string

type stockbitTableLayout struct{}

func (*stockbitTableLayout) Name() string   { return "stockbit_table_layout" }
func (*stockbitTableLayout) SkipProd() bool { return false }

func stockbitOriginalLayout() (string, error) {
	var templates []models.PDFParsingTemplate
	if err := json.Unmarshal(metadataTradeTemplates, &templates); err != nil {
		return "", err
	}
	for _, t := range templates {
		if t.Key == "stockbit" && t.Version == 2 {
			var config struct {
				Trade struct {
					Extraction string          `json:"extraction"`
					Fees       json.RawMessage `json:"fees"`
				} `json:"trade"`
			}
			if err := json.Unmarshal(t.Config, &config); err != nil {
				return "", err
			}
			data, err := json.Marshal(config.Trade)
			return string(data), err
		}
	}
	return "", errors.New("missing stockbit v2 seed")
}

func replaceStockbitLayout(tx *sql.Tx, from, to string) error {
	_, err := tx.Exec(`UPDATE pdf_parsing_templates
		SET config=jsonb_set(config, '{trade}', (config->'trade') || $2::jsonb), updated_at=NOW()
		WHERE key='stockbit' AND version=2 AND config->>'parse_mode'='metadata'
		AND (config->'trade') @> $1::jsonb AND config#>'{trade,fees}'=$1::jsonb->'fees'`, from, to)
	return err
}

func (*stockbitTableLayout) Up(tx *sql.Tx) error {
	old, err := stockbitOriginalLayout()
	if err != nil {
		return err
	}
	return replaceStockbitLayout(tx, old, stockbitLayoutPatch)
}
func (*stockbitTableLayout) Down(tx *sql.Tx) error {
	old, err := stockbitOriginalLayout()
	if err != nil {
		return err
	}
	return replaceStockbitLayout(tx, stockbitLayoutPatch, old)
}
