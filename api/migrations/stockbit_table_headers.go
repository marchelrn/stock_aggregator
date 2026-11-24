package migrations

import (
	"database/sql"
	_ "embed"
)

//go:embed templates/stockbit_table_headers.json
var stockbitHeaders string

type stockbitTableHeaders struct{}

func (*stockbitTableHeaders) Name() string   { return "stockbit_table_headers" }
func (*stockbitTableHeaders) SkipProd() bool { return false }

func updateStockbitHeaders(tx *sql.Tx, reverse bool) error {
	_, err := tx.Exec(`
	WITH original AS (
		SELECT jsonb_build_object(
			'required_markers', item#>'{config,required_markers}',
			'start_pattern', item#>'{config,table,start_pattern}',
			'date_pattern', item#>'{config,date,pattern}') AS headers
		FROM jsonb_array_elements($1::jsonb) item WHERE item->>'key'='stockbit'
	), changes AS (
		SELECT CASE WHEN $3 THEN $2::jsonb ELSE headers END AS prev,
			CASE WHEN $3 THEN headers ELSE $2::jsonb END AS next FROM original
	)
	UPDATE pdf_parsing_templates SET updated_at=NOW(),
		config=jsonb_set(jsonb_set(jsonb_set(config,
			'{required_markers}', next->'required_markers'),
			'{table,start_pattern}', next->'start_pattern'),
			'{date,pattern}', next->'date_pattern')
	FROM changes WHERE key='stockbit' AND version=2 AND config->>'parse_mode'='metadata'
		AND config->'required_markers'=prev->'required_markers'
		AND config#>'{table,start_pattern}'=prev->'start_pattern'
		AND config#>'{date,pattern}'=prev->'date_pattern'`, string(metadataTradeTemplates), stockbitHeaders, reverse)
	return err
}

func (*stockbitTableHeaders) Up(tx *sql.Tx) error   { return updateStockbitHeaders(tx, false) }
func (*stockbitTableHeaders) Down(tx *sql.Tx) error { return updateStockbitHeaders(tx, true) }
