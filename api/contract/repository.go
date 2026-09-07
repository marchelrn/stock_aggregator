package contract

import (
	"github.com/marchelrn/stock_api/models"
)

type Repository struct {
	Stocks       StocksRepository
	Brokers      BrokerRepository
	Transactions TransactionRepository
	Yahoo        YahooRepository
	Auth         AuthRepository
}

type AuthRepository interface {
	FindByEmail(email string) (*models.Users, error)
	FindByID(id uint) (*models.Users, error)
	FindByProviderID(providerID string) (*models.Users, error)
	CreateUser(user *models.Users) (*models.Users, error)
	UpdateUser(user *models.Users) (*models.Users, error)
}

type StocksRepository interface {
	GetAllStocks(limiter int) ([]models.StockHolding, error)
	GetStocks(ticker []string) ([]models.StockHolding, error)
	GetStocksReturnModels(ticker []string) (*models.StockHolding, error)
	GetStockByTickerAndBroker(ticker string, brokerID int) (*models.StockHolding, error)
	GetStockById(id int) (*models.StockHolding, error)
	AddStockData(payload *models.StockHolding) (*models.StockHolding, error)
	UpdateStockData(ticker []string, payload *models.StockHolding) (*models.StockHolding, error)
	DeleteStockData(stockTicker []string) error
}

type BrokerRepository interface {
	GetBrokerById(id int) (*models.Broker, error)
	GetBrokerDetails(brokerName []string) ([]models.BrokerDetails, error)
	GetAllBrokerHoldings() ([]models.StockHolding, error)
	AddBrokerData(payload *models.Broker) (*models.Broker, error)
	DeleteBrokerData(brokerName []string) (*models.Broker, error)
	GetAllBrokers() ([]models.Broker, error)
	GetBrokerByName(name string) (*models.Broker, error)
	UpdateBrokerData(id int, payload *models.Broker) (*models.Broker, error)
}

type TransactionRepository interface {
	GetTransactions(id int) ([]models.Transaction, error)
	GetAllTransactions() ([]models.Transaction, error)
	CreateTransactionAndSyncHolding(payload *models.Transaction) (*models.Transaction, error)
	CountTransactionsByTicker(ticker string) (int64, error)
	DeleteTransaction(id int) error
}

type YahooRepository interface {
	GetPrice(ticker string) (*models.StockPrice, error)
	GetPrices(tickers []string) (map[string]*models.StockPrice, error)
	GetAllPrices() ([]models.StockPrice, error)
	SavePrice(price *models.StockPrice) error
}
