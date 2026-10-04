package dto

import "time"

// Stocks Struct

type StocksRequest struct {
	Ticker     string  `json:"ticker" `
	Lot        int     `json:"lot"`
	AvgPrice   float64 `json:"avg_price"`
	BrokerID   int     `json:"broker_id"`
	BrokerName string  `json:"broker_name"`
}

type StocksResponse struct {
	StatusCode int          `json:"status_code"`
	Message    string       `json:"message"`
	StocksData []StocksData `json:"stocks_data"`
}

type StocksData struct {
	Id         int       `json:"id"`
	Ticker     string    `json:"ticker"`
	Lot        int       `json:"lot"`
	AvgPrice   float64   `json:"avg_price"`
	BrokerID   int       `json:"broker_id"`
	BrokerName string    `json:"broker_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Brokers Struct

type BrokersResponse struct {
	StatusCode  int         `json:"status_code"`
	Message     string      `json:"message"`
	BrokersData BrokersData `json:"brokers_data"`
}

type BasicBrokerResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Data       Broker `json:"data"`
}

type BasicBrokersResponse struct {
	StatusCode int      `json:"status_code"`
	Message    string   `json:"message"`
	Data       []Broker `json:"data"`
}

type BrokersData struct {
	Brokers   []BrokerDetails `json:"brokers"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type BrokerRequest struct {
	Id   int     `json:"id"`
	Name string  `json:"name" `
	Cash float64 `json:"cash"`
}

type Broker struct {
	Id   int     `json:"id"`
	Name string  `json:"name"`
	Cash float64 `json:"cash"`
}

type BrokerDetails struct {
	Id          int          `json:"id"`
	Name        string       `json:"name"`
	Cash        float64      `json:"cash"`
	StocksData  []StocksData `json:"stocks_data"`
	StocksCount int          `json:"stocks_count"`
}

// Transactions struct

type BrokerTransaction struct {
	BrokerID int    `json:"broker_id"`
	Name     string `json:"name"`
}

type TransferCashRequest struct {
	FromBrokerId int     `json:"from_broker_id"`
	ToBrokerId   int     `json:"to_broker_id"`
	Amount       float64 `json:"amount"`
}

type StockTransaction struct {
	Id     int     `json:"id"`
	Ticker string  `json:"ticker"`
	Lot    int     `json:"lot"`
	Price  float64 `json:"price"`
}

type TransactionData struct {
	Id         int               `json:"id"`
	Broker     BrokerTransaction `json:"broker"`
	Stock      StockTransaction  `json:"stock"`
	Type       string            `json:"type"`
	AmountDone float64           `json:"amount_done"`
	Date       time.Time         `json:"date"`
}

type TransactionRequest struct {
	Id         int       `json:"id"`
	Ticker     string    `json:"ticker"`
	Price      float64   `json:"price"`
	Lot        int       `json:"lot"`
	StockID    int       `json:"stock_id"`
	BrokerID   int       `json:"broker_id"`
	BrokerName string    `json:"broker_name"`
	Type       string    `json:"type"`
	Date       time.Time `json:"date"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TransactionResponse struct {
	StatusCode int             `json:"status_code"`
	Message    string          `json:"message"`
	Data       TransactionData `json:"data"`
}

type AllTransactionsResponse struct {
	StatusCode int               `json:"status_code"`
	Message    string            `json:"message"`
	Data       []TransactionData `json:"data"`
}

// User Struct

type UserData struct {
	Id           uint      `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	AuthProvider string    `json:"auth_provider"`
	ProviderID   *string   `json:"provider_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DeletedAt    time.Time `json:"deleted_at"`
}

type UserResponse struct {
	StatusCode int      `json:"status_code"`
	Message    string   `json:"message"`
	Data       UserData `json:"data"`
}

// Auth Struct

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	StatusCode int      `json:"status_code"`
	Message    string   `json:"message"`
	Token      string   `json:"token"`
	Data       UserData `json:"data"`
}

// Basic response struct

type BasicResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

// Gmail Sync Struct

type ParsedTrade struct {
	Ticker     string    `json:"ticker"`
	Lot        int       `json:"lot"`
	Price      float64   `json:"price"`
	Type       string    `json:"type"` // BUY or SELL
	Date       time.Time `json:"date"`
	BrokerName string    `json:"broker_name"`
	AmountDone float64   `json:"amount_done"`
}

type GmailSyncResponse struct {
	StatusCode   int           `json:"status_code"`
	Message      string        `json:"message"`
	TradesFound  int           `json:"trades_found"`
	TradesSynced int           `json:"trades_synced"`
	ParsedTrades []ParsedTrade `json:"parsed_trades"`
}

type GmailSyncStatusResponse struct {
	StatusCode       int        `json:"status_code"`
	Message          string     `json:"message"`
	GmailSyncEnabled bool       `json:"gmail_sync_enabled"`
	LastSyncAt       *time.Time `json:"last_sync_at"`
}
