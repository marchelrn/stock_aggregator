package dto

import "time"

type PortfolioPerformancePoint struct {
	Date             string    `json:"date"`
	BrokerID         int       `json:"broker_id"`
	BrokerName       string    `json:"broker_name"`
	TotalValue       *float64  `json:"total_value"`
	UnpricedHoldings int       `json:"unpriced_holdings"`
	CapturedAt       time.Time `json:"captured_at"`
}
