package service

import (
	"fmt"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/service/parser"
)

func New(repo *contract.Repository) (*contract.Service, error) {
	if repo == nil || repo.PDFTemplates == nil {
		return nil, fmt.Errorf("PDF template repository is required")
	}
	metadataParser := parser.NewMetadataParser(repo.PDFTemplates)
	stockbitParser := parser.NewStockbitParser()
	mandiriParser := parser.NewMandiriParser()
	dispatcher := parser.NewBrokerDispatcherParser(metadataParser, stockbitParser, mandiriParser)

	yahooSvc := ImplYahooService(repo.Yahoo)

	return &contract.Service{
		Stocks:      ImplStocksService(repo.Stocks, repo.Brokers, repo.Transactions, yahooSvc),
		Brokers:     ImplBrokersService(repo.Brokers),
		Transaction: ImplTransactionService(repo),
		Yahoo:       yahooSvc,
		Auth:        ImplAuthService(repo.Auth),
		Gmail:       ImplGmailService(repo.Auth, dispatcher, repo.Transactions, repo.CashMutations, repo.Brokers),
	}, nil
}
