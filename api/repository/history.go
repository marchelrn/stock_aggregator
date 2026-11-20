package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/marchelrn/stock_api/dto"
	"gorm.io/gorm"
)

const historyRows = `
	SELECT 'trade' AS kind, id, date, type, broker_name, ticker, lot, price, amount_done AS amount, tax
	FROM transactions WHERE user_id = ?
	AND strpos(lower(concat_ws(chr(10), ticker, broker_name, type)), lower(?)) > 0
	UNION ALL
	SELECT 'cash', id, date, type, broker_name, '', 0, 0, amount, 0
	FROM cash_mutations WHERE user_id = ?
	AND strpos(lower(concat_ws(chr(10), broker_name, type, description, ref_no)), lower(?)) > 0`

func (r *TransactionRepository) History(ctx context.Context, userID uint, page, size int, search string) (*dto.HistoryPage, error) {
	search = strings.TrimSpace(search)
	args := []any{userID, search, userID, search}
	result := &dto.HistoryPage{Data: []dto.HistoryItem{}, Page: page}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw("SELECT count(*) FROM ("+historyRows+") h", args...).Scan(&result.Total).Error; err != nil {
			return err
		}
		last := max(1, (result.Total+size-1)/size)
		result.Page = min(page, last)
		return tx.Raw(`SELECT kind, id, type, broker_name, ticker, lot, price, amount, tax,
			CASE WHEN kind = 'cash' AND date::time = TIME '00:00'
				THEN to_char(date, 'YYYY-MM-DD')
				ELSE to_char(date, 'YYYY-MM-DD"T"HH24:MI:SS.US') || '+08:00'
			END AS date
			FROM (`+historyRows+`) h
			ORDER BY h.date DESC, kind, id DESC LIMIT ? OFFSET ?`,
			append(args, size, (result.Page-1)*size)...).Scan(&result.Data).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}
