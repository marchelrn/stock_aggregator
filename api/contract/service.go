package contract

import (
	"context"

	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
)

type Service struct {
	Stocks      StocksService
	Brokers     BrokersService
	Transaction TransactionService
	Yahoo       YahooService
	Auth        AuthService
	Gmail       GmailService
}

type AuthService interface {
	Register(payload *dto.RegisterRequest) (*dto.AuthResponse, error)
	Login(payload *dto.LoginRequest) (*dto.AuthResponse, error)
	GoogleLogin(code string) (*dto.AuthResponse, error)
	GetGoogleLoginURL(state string) string
	GetUserInformation(userID float64) (*dto.UserResponse, error)
}

type GmailService interface {
	SyncTradeConfirmations(userID uint) (*dto.GmailSyncResponse, error)
	GetSyncStatus(userID uint) (*dto.GmailSyncStatusResponse, error)
}

type StocksService interface {
	GetAllStocks(limiter int) (*dto.StocksResponse, error)
	GetStocks(stockTicker []string) (*dto.StocksResponse, error)
	AddStockData(payload *dto.StocksRequest) (*dto.StocksResponse, error)
	UpdateStock(stockTicker []string, payload *dto.StocksRequest) (*dto.StocksResponse, error)
	DeleteStockData(stockTicker []string) (*dto.StocksResponse, error)
}

type BrokersService interface {
	GetAllBrokersDetails() (*dto.BrokersResponse, error)
	GetBroker(id int) (*dto.BasicBrokerResponse, error)
	GetAllBrokers() (*dto.BasicBrokersResponse, error)
	GetBrokersDetails(brokersName []string) (*dto.BrokersResponse, error)
	AddBrokerData(payload *dto.BrokerRequest) (*dto.BrokersResponse, error)
	DeleteBrokerData(brokerName []string) (*dto.BasicResponse, error)
	UpdateBrokerData(id int, payload *dto.BrokerRequest) (*dto.BasicResponse, error)
	TransferCashBetweenBrokers(payload *dto.TransferCashRequest) (*dto.BasicResponse, error)
}

type TransactionService interface {
	GetTransaction(id int) (*dto.TransactionResponse, error)
	GetAllTransactions() (*dto.AllTransactionsResponse, error)
	CreateTransaction(ticker []string, payload *dto.TransactionRequest) (*dto.TransactionResponse, error)
	DeleteTransaction(id int) (*dto.BasicResponse, error)
}

type YahooService interface {
	GetMarket(ctx context.Context, ticker string) (*dto.MarketResponse, error)
	GetPrice(ticker string) (*models.StockPrice, error)
	GetPrices(tickers []string) (prices map[string]*models.StockPrice, failed map[string]string, err error)
	GetAllPrices() ([]models.StockPrice, error)
}
