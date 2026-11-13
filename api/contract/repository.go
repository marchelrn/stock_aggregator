package contract

import (
	"context"

	"github.com/marchelrn/stock_api/models"
)

type Repository struct {
	Stocks           StocksRepository
	Brokers          BrokerRepository
	Transactions     TransactionRepository
	CashMutations    CashMutationRepository
	Yahoo            YahooRepository
	Auth             AuthRepository
	PDFTemplates     PDFTemplateRepository
}

type PDFTemplateRepository interface {
	ListActive(context.Context) ([]models.PDFParsingTemplate, error)
}

type AuthRepository interface {
	FindByEmail(email string) (*models.Users, error)
	FindByID(id uint) (*models.Users, error)
	FindByIDFloat64(id float64) (*models.Users, error)
	FindByProviderID(providerID string) (*models.Users, error)
	CreateUser(user *models.Users) (*models.Users, error)
	UpdateUser(user *models.Users) (*models.Users, error)
}

type StocksRepository interface {
	GetAllStocks(userID uint, limiter int) ([]models.StockHolding, error)
	GetStocks(userID uint, ticker []string) ([]models.StockHolding, error)
	GetStocksReturnModels(userID uint, ticker []string) (*models.StockHolding, error)
	GetStockByTickerAndBroker(userID uint, ticker string, brokerID int) (*models.StockHolding, error)
	GetStockById(userID uint, id int) (*models.StockHolding, error)
	AddStockData(payload *models.StockHolding) (*models.StockHolding, error)
	UpdateStockData(userID uint, ticker []string, payload *models.StockHolding) (*models.StockHolding, error)
	DeleteStockData(userID uint, stockTicker []string) error
	UpdateStockSector(userID uint, ticker string, sector string) error
	UpdateSectorByTicker(ticker string, sector string) error
}

type BrokerRepository interface {
	GetBrokerById(userID uint, id int) (*models.Broker, error)
	GetBrokerDetails(userID uint, brokerName []string) ([]models.BrokerDetails, error)
	GetAllBrokerHoldings(userID uint) ([]models.StockHolding, error)
	AddBrokerData(payload *models.Broker) (*models.Broker, error)
	DeleteBrokerData(userID uint, brokerName []string) (*models.Broker, error)
	GetAllBrokers(userID uint) ([]models.Broker, error)
	GetBrokerByName(userID uint, name string) (*models.Broker, error)
	UpdateBrokerData(userID uint, id int, payload *models.Broker) (*models.Broker, error)
}

type TransactionRepository interface {
	GetTransactions(userID uint, id int) ([]models.Transaction, error)
	GetAllTransactions(userID uint) ([]models.Transaction, error)
	CreateTransactionAndSyncHolding(payload *models.Transaction) (*models.Transaction, error)
	SyncTradeConfirmationBatch(userID uint, trades []*models.Transaction) (int, error)
	CountTransactionsByTicker(userID uint, ticker string) (int64, error)
	DeleteTransaction(userID uint, id int) error
}

type CashMutationRepository interface {
	GetAll(userID uint) ([]models.CashMutation, error)
	SyncCashMutationBatch(userID uint, mutations []*models.CashMutation) (int, error)
}

type YahooRepository interface {
	GetPrice(ticker string) (*models.StockPrice, error)
	GetPrices(tickers []string) (map[string]*models.StockPrice, error)
	GetAllPrices() ([]models.StockPrice, error)
	SavePrice(price *models.StockPrice) error
	UpdateSector(ticker string, sector string) error
}
