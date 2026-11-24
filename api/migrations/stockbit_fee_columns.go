package migrations

import (
	"database/sql"
	_ "embed"
)

//go:embed templates/stockbit_fee_columns.json
var stockbitFeeColumnsPatch string

type stockbitFeeColumns struct{}

func (*stockbitFeeColumns) Name() string   { return "stockbit_fee_columns" }
func (*stockbitFeeColumns) SkipProd() bool { return false }

func (*stockbitFeeColumns) Up(tx *sql.Tx) error {
	return replaceStockbitLayout(tx, stockbitLayoutPatch, stockbitFeeColumnsPatch)
}

func (*stockbitFeeColumns) Down(tx *sql.Tx) error {
	return replaceStockbitLayout(tx, stockbitFeeColumnsPatch, stockbitLayoutPatch)
}
