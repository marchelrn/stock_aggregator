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
	GetAllStocks(userID uint, limiter int) (*dto.StocksResponse, error)
	GetStocks(userID uint, stockTicker []string) (*dto.StocksResponse, error)
	AddStockData(userID uint, payload *dto.StocksRequest) (*dto.StocksResponse, error)
	UpdateStock(userID uint, stockTicker []string, payload *dto.StocksRequest) (*dto.StocksResponse, error)
	DeleteStockData(userID uint, stockTicker []string) (*dto.StocksResponse, error)
	SyncStockSectors(userID uint) (*dto.StocksResponse, error)
}

type BrokersService interface {
	GetAllBrokersDetails(userID uint) (*dto.BrokersResponse, error)
	GetBroker(userID uint, id int) (*dto.BasicBrokerResponse, error)
	GetAllBrokers(userID uint) (*dto.BasicBrokersResponse, error)
	GetBrokersDetails(userID uint, brokersName []string) (*dto.BrokersResponse, error)
	AddBrokerData(userID uint, payload *dto.BrokerRequest) (*dto.BrokersResponse, error)
	DeleteBrokerData(userID uint, brokerName []string) (*dto.BasicResponse, error)
	UpdateBrokerData(userID uint, id int, payload *dto.BrokerRequest) (*dto.BasicResponse, error)
	TransferCashBetweenBrokers(userID uint, payload *dto.TransferCashRequest) (*dto.BasicResponse, error)
}

type TransactionService interface {
	GetTransaction(userID uint, id int) (*dto.TransactionResponse, error)
	GetAllTransactions(userID uint) (*dto.AllTransactionsResponse, error)
	CreateTransaction(userID uint, ticker []string, payload *dto.TransactionRequest) (*dto.TransactionResponse, error)
	DeleteTransaction(userID uint, id int) (*dto.BasicResponse, error)
}

type YahooService interface {
	GetMarket(ctx context.Context, ticker string) (*dto.MarketResponse, error)
	GetPrice(ticker string) (*models.StockPrice, error)
	GetPrices(tickers []string) (prices map[string]*models.StockPrice, failed map[string]string, err error)
	GetAllPrices() ([]models.StockPrice, error)
	GetSector(ticker string) (string, error)
	GetSectors(tickers []string) (map[string]string, error)
}
