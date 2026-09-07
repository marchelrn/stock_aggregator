package repository

import (
	"github.com/marchelrn/stock_api/contract"

	"gorm.io/gorm"
)

func New(db *gorm.DB) *contract.Repository {
	return &contract.Repository{
		Stocks:       ImplStocksRepository(db),
		Brokers:      ImplBrokersRepository(db),
		Transactions: ImplTransactionRepository(db),
		Yahoo:        ImplYahooRepository(db),
		Auth:         ImplAuthRepository(db),
	}
}
