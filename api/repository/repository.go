package repository

import (
	"github.com/marchelrn/stock_api/contract"

	"gorm.io/gorm"
)

func New(db *gorm.DB) *contract.Repository {
	return &contract.Repository{
		Rights:        rightExercises{db},
		Reviews:       &TradeReviewRepository{db},
		Stocks:        ImplStocksRepository(db),
		Brokers:       ImplBrokersRepository(db),
		Transactions:  ImplTransactionRepository(db),
		CashMutations: ImplCashMutationRepository(db),
		Yahoo:         ImplYahooRepository(db),
		Auth:          ImplAuthRepository(db),
		PDFTemplates:  ImplPDFTemplateRepository(db),
		GmailCache:    ImplGmailCacheRepository(db),
	}
}
