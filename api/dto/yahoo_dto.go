package dto

import (
	"net/http"
	"sync"
	"time"

	"github.com/marchelrn/stock_api/models"
)

type YahooResponse struct {
	Chart Chart `json:"chart"`
}

type Chart struct {
	Result []Result `json:"result"`
	Error  *Error   `json:"error"`
}

type Result struct {
	Meta Meta `json:"meta"`
}

type Error struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type Meta struct {
	Symbol             string  `json:"symbol"`
	RegularMarketPrice float64 `json:"regularMarketPrice"`
	PreviousClose      float64 `json:"previousClose"`
	Currency           string  `json:"currency"`
}

type YahooService struct {
	Client *http.Client
	Cache  map[string]*CachedPrice
	Mu     *sync.RWMutex
}

type CachedPrice struct {
	Price     *models.StockPrice
	FetchedAt time.Time
}
