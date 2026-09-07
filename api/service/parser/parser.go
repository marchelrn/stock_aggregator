package parser

import "github.com/marchelrn/stock_api/dto"

// TradeParser is the common interface for all broker-specific PDF parsers
type TradeParser interface {
	Parse(pdfBytes []byte) ([]dto.ParsedTrade, error)
	BrokerName() string
}
