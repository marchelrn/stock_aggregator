package migrations

import (
	"database/sql"
	_ "embed"
)

//go:embed templates/trimegah_statement_filter.json
var trimegahStatementFilter string

type excludeTrimegahStatements struct{}

func (*excludeTrimegahStatements) Name() string   { return "exclude_trimegah_statements" }
func (*excludeTrimegahStatements) SkipProd() bool { return false }

const trimegahTradeVersions = `key='trimegah' AND version IN (1, 2)
	AND config->>'kind'='trade' AND config->>'document'='pdf'`

func (*excludeTrimegahStatements) Up(tx *sql.Tx) error {
	_, err := tx.Exec(`UPDATE pdf_parsing_templates SET config=config || $1::jsonb, updated_at=NOW()
		WHERE `+trimegahTradeVersions+` AND NOT (config ? 'exclude_filename_pattern')`, trimegahStatementFilter)
	return err
}

func (*excludeTrimegahStatements) Down(tx *sql.Tx) error {
	_, err := tx.Exec(`UPDATE pdf_parsing_templates SET config=config - 'exclude_filename_pattern', updated_at=NOW()
		WHERE `+trimegahTradeVersions+` AND config @> $1::jsonb`, trimegahStatementFilter)
	return err
}
