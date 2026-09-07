package models

import "time"

// Brokers
type Broker struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Cash      float64   `json:"cash"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BrokerDetails struct {
	Broker
	Stocks []StockHolding `json:"stocks" gorm:"foreignKey:BrokerID;references:ID"`
}

// Stocks
type StockHolding struct {
	Id         int       `json:"stock_id" gorm:"column:id;primaryKey"`
	Ticker     string    `json:"ticker"`
	Lot        int       `json:"lot"`
	AvgPrice   float64   `json:"avg_price"`
	BrokerID   int       `json:"broker_id"`
	BrokerName string    `json:"broker_name"`
	Broker     Broker    `json:"broker" gorm:"foreignKey:BrokerID;references:ID"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	DeletedAt  time.Time `json:"deleted_at" gorm:"index"`
}

type StockPrice struct {
	Id            int       `json:"id"`
	Ticker        string    `json:"ticker"`
	Price         float64   `json:"price"`
	PreviousClose float64   `json:"previous_close"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	Currency      string    `json:"currency"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Transactions

type Transaction struct {
	Id         int       `json:"transaction_id"`
	Ticker     string    `json:"ticker"`
	Price      float64   `json:"price"`
	Lot        int       `json:"lot"`
	StockID    int       `json:"stock_id"`
	BrokerID   int       `json:"broker_id"`
	BrokerName string    `json:"broker_name"`
	Type       Type      `json:"type" gorm:"type:enum('BUY', 'SELL')"`
	Date       time.Time `json:"date"`
	AmountDone float64   `json:"amount_done"`
}

type Type string

const (
	TypeBuy  Type = "BUY"
	TypeSell Type = "SELL"
)

// Portfolio

type Portfolio struct {
	ID         int             `json:"id"`
	Brokers    []BrokerDetails `json:"brokers"`
	Holdings   []StockHolding  `json:"holdings"`
	Cash       float64         `json:"cash"`
	Invested   float64         `json:"invested"`
	Total      float64         `json:"total"`
	Percentage float64         `json:"percentage"`
}

// Users

type Users struct {
	Id                uint       `json:"id"`
	Email             string     `json:"email"`
	Name              string     `json:"name"`
	Password          *string    `json:"-"`
	AuthProvider      string     `json:"auth_provider"`
	ProviderID        *string    `json:"provider_id"`
	GmailAccessToken  *string    `json:"-" gorm:"column:gmail_access_token"`
	GmailRefreshToken *string    `json:"-" gorm:"column:gmail_refresh_token"`
	GmailTokenExpiry  *time.Time `json:"-" gorm:"column:gmail_token_expiry"`
	GmailSyncEnabled  bool       `json:"gmail_sync_enabled" gorm:"column:gmail_sync_enabled;default:false"`
	LastGmailSyncAt   *time.Time `json:"last_gmail_sync_at" gorm:"column:last_gmail_sync_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         time.Time  `json:"deleted_at" gorm:"index"`
}

// Table Names

func (StockHolding) TableName() string {
	return "stocks"
}

func (Broker) TableName() string {
	return "brokers"
}

func (StockPrice) TableName() string {
	return "stock_prices"
}
