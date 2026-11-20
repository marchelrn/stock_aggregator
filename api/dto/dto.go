package dto

import "time"

// Stocks Struct

type StocksRequest struct {
	Ticker     string  `json:"ticker" `
	Sector     string  `json:"sector,omitempty"`
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
	Sector     string    `json:"sector"`
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
	Pin  string  `json:"pin"`
	Id   int     `json:"id"`
	Name string  `json:"name" `
	Cash float64 `json:"cash"`
}

type Broker struct {
	Id           int        `json:"id"`
	Name         string     `json:"name"`
	Cash         float64    `json:"cash"`
	CashBase     float64    `json:"cash_base"`
	CashBaseAsOf *time.Time `json:"cash_base_as_of"`
}

type BrokerDetails struct {
	Id           int          `json:"id"`
	Name         string       `json:"name"`
	Cash         float64      `json:"cash"`
	CashBase     float64      `json:"cash_base"`
	CashBaseAsOf *time.Time   `json:"cash_base_as_of"`
	StocksData   []StocksData `json:"stocks_data"`
	StocksCount  int          `json:"stocks_count"`
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

// SetCashBaseRequest sets the current balance, effective at server time.
type SetCashBaseRequest struct {
	CashBase float64 `json:"cash_base"`
	// Deprecated: accepted for compatibility but clamped to server now.
	CashBaseAsOf *time.Time `json:"cash_base_as_of"`
}

// CashEvent adalah satu baris riwayat yang memengaruhi cash setelah cutoff.
type CashEvent struct {
	Source      string    `json:"source"` // TRADE | MUTATION
	Type        string    `json:"type"`   // BUY/SELL/DEPOSIT/WITHDRAW/DIVIDEND
	Delta       float64   `json:"delta"`
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
}

type CashBreakdown struct {
	BrokerId     int         `json:"broker_id"`
	BrokerName   string      `json:"broker_name"`
	Cash         float64     `json:"cash"`
	CashBase     float64     `json:"cash_base"`
	CashBaseAsOf *time.Time  `json:"cash_base_as_of"`
	EventsTotal  float64     `json:"events_total"`
	Events       []CashEvent `json:"events"`
}

type CashBreakdownResponse struct {
	StatusCode int           `json:"status_code"`
	Message    string        `json:"message"`
	Data       CashBreakdown `json:"data"`
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
	Tax        float64           `json:"tax"`
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
	Tax        float64   `json:"tax"`
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
	Tax        float64   `json:"tax"`
	RefNo      string    `json:"ref_no,omitempty"`
}

type GmailSyncResponse struct {
	StatusCode   int           `json:"status_code"`
	Message      string        `json:"message"`
	TradesFound  int           `json:"trades_found"`
	TradesSynced int           `json:"trades_synced"`
	ParsedTrades []ParsedTrade `json:"parsed_trades"`
}

const GmailOAuthStatePrefix = "gmail."

type GmailConnectionStart struct {
	URL      string `json:"url"`
	State    string `json:"state"`
	Verifier string `json:"verifier"`
}

type GmailConnectionRequest struct {
	Code     string `json:"code" binding:"required"`
	State    string `json:"state" binding:"required"`
	Verifier string `json:"verifier" binding:"required,len=43"`
}

type GmailSyncStatusResponse struct {
	StatusCode       int        `json:"status_code"`
	Message          string     `json:"message"`
	GmailSyncEnabled bool       `json:"gmail_sync_enabled"`
	LastSyncAt       *time.Time `json:"last_sync_at"`
}

type ParsedCashMutation struct {
	BrokerName  string  `json:"broker_name"`
	Type        string  `json:"type"` // DEPOSIT, WITHDRAW, or DIVIDEND
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
	RefNo       string  `json:"ref_no,omitempty"`
	Description string  `json:"description,omitempty"`
	// EmailReceivedAt is receipt metadata only; Date controls cash eligibility.
	EmailReceivedAt *time.Time `json:"email_received_at,omitempty"`
}

type GmailCashSyncResponse struct {
	StatusCode      int                  `json:"status_code"`
	Message         string               `json:"message"`
	MutationsFound  int                  `json:"mutations_found"`
	MutationsSynced int                  `json:"mutations_synced"`
	ParsedMutations []ParsedCashMutation `json:"parsed_mutations"`
}

type CashMutationItem struct {
	ID          int     `json:"id"`
	UserID      int     `json:"user_id"`
	BrokerID    int     `json:"broker_id"`
	BrokerName  string  `json:"broker_name"`
	Type        string  `json:"type"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
	Description string  `json:"description"`
	RefNo       string  `json:"ref_no"`
	CreatedAt   string  `json:"created_at"`
}

type CashMutationsListResponse struct {
	StatusCode int                `json:"status_code"`
	Message    string             `json:"message"`
	Data       []CashMutationItem `json:"data"`
}

type HistoryItem struct {
	Kind       string  `json:"kind"`
	ID         int     `json:"id"`
	Type       string  `json:"type"`
	BrokerName string  `json:"broker_name"`
	Date       string  `json:"date"`
	Ticker     string  `json:"ticker,omitempty"`
	Lot        int     `json:"lot,omitempty"`
	Price      float64 `json:"price,omitempty"`
	Amount     float64 `json:"amount"`
	Tax        float64 `json:"tax"`
}

type HistoryPage struct {
	Data  []HistoryItem `json:"data"`
	Page  int           `json:"page"`
	Total int           `json:"total"`
}
