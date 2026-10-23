package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/service/parser"
)

type fallbackReport struct {
	Provider    string                 `json:"provider"`
	Trades      []dto.ParsedTrade      `json:"trades,omitempty"`
	Attempted   bool                   `json:"attempted"`
	Model       string                 `json:"model"`
	Status      string                 `json:"status"`
	Parsed      int                    `json:"parsed"`
	Missing     int                    `json:"missing"`
	Unexpected  int                    `json:"unexpected"`
	DurationMS  int64                  `json:"duration_ms"`
	Error       string                 `json:"error,omitempty"`
	Diagnostics *completionDiagnostics `json:"diagnostics,omitempty"`
}

// The parser has no typed document errors. Keep this allowlist conservative:
// unknown errors (especially store/configuration failures) never invoke AI.
func shouldFallback(got []dto.ParsedTrade, err error) bool {
	if errors.Is(err, parser.ErrEmailNotApplicable) {
		return false
	}
	if err == nil {
		return len(got) == 0
	}
	s := err.Error()
	for _, prefix := range []string{"unmatched PDF:", "PDF template \"", "cannot read PDF", "PDF extraction failed", "PDF page ", "cannot extract PDF page ", "PDF contains no extractable text"} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

type aiTrade struct {
	Reference   *string `json:"reference"`
	Ticker      *string `json:"ticker"`
	Side        *string `json:"side"`
	TradeDate   *string `json:"trade_date"`
	Lot         *int64  `json:"lot"`
	Quantity    *int64  `json:"quantity"`
	Price       *string `json:"price"`
	GrossAmount *string `json:"gross_amount"`
}

func tradeSchema() map[string]any {
	props := map[string]any{}
	for _, key := range []string{"reference", "ticker", "trade_date", "price", "gross_amount"} {
		props[key] = map[string]any{"type": []string{"string", "null"}}
	}
	props["trade_date"].(map[string]any)["pattern"] = `^\d{4}-\d{2}-\d{2}$`
	props["side"] = map[string]any{"type": []string{"string", "null"}, "enum": []any{"BUY", "SELL", nil}}
	for _, key := range []string{"lot", "quantity"} {
		props[key] = map[string]any{"type": []string{"integer", "null"}}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"trades"}, "properties": map[string]any{"trades": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"reference", "ticker", "side", "trade_date", "lot", "quantity", "price", "gross_amount"}, "properties": props}}}}
}

func validateAITrades(content string) ([]dto.ParsedTrade, error) {
	invalid := errors.New("AI output failed strict trade validation")
	var result struct {
		Trades []aiTrade `json:"trades"`
	}
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, invalid
	}
	if decoder.Decode(new(any)) != io.EOF || len(result.Trades) == 0 {
		return nil, invalid
	}
	trades := make([]dto.ParsedTrade, 0, len(result.Trades))
	for index, t := range result.Trades {
		for _, field := range []struct {
			name    string
			missing bool
		}{
			{"ticker", t.Ticker == nil}, {"side", t.Side == nil}, {"trade_date", t.TradeDate == nil},
			{"lot", t.Lot == nil}, {"quantity", t.Quantity == nil}, {"price", t.Price == nil}, {"gross_amount", t.GrossAmount == nil},
		} {
			if field.missing {
				return nil, fmt.Errorf("AI row %d: missing/null %s", index+1, field.name)
			}
		}
		if !tickerPattern.MatchString(*t.Ticker) {
			return nil, fmt.Errorf("AI row %d: invalid ticker", index+1)
		}
		date, err := time.Parse("2006-01-02", *t.TradeDate)
		if err != nil || date.Format("2006-01-02") != *t.TradeDate {
			return nil, fmt.Errorf("AI row %d: invalid trade_date", index+1)
		}
		buy, sell := "0", "0"
		switch *t.Side {
		case "BUY":
			buy = *t.GrossAmount
		case "SELL":
			sell = *t.GrossAmount
		default:
			return nil, invalid
		}
		// Reference/board are not compared. Placeholders only adapt the shared
		// arithmetic converter; no transaction field is inferred or filled in.
		row := []string{"0", "unused", *t.Ticker, strconv.FormatInt(*t.Lot, 10), strconv.FormatInt(*t.Quantity, 10), *t.Price, buy, sell, date.Format("02/01/2006")}
		converted, err := parseGroundTruthRow(row)
		if err != nil {
			return nil, fmt.Errorf("AI row %d: %w", index+1, err)
		}
		trades = append(trades, converted.Trade)
	}
	return trades, nil
}

// Only allowlisted labels and numeric metadata may leave the response decoder.
// Reasoning is never used as final content, even if it contains valid trade JSON.
type completionDiagnostics struct {
	Envelope         string `json:"envelope"`
	CompletionReason string `json:"completion_reason"`
	ChoiceCount      int    `json:"choice_count"`
	ContentBytes     int    `json:"content_bytes"`
	ReasoningBytes   int    `json:"reasoning_bytes"`
	PromptTokens     *int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens *int64 `json:"completion_tokens,omitempty"`
}

func safeTokenCount(n *int64) *int64 {
	if n == nil || *n < 0 {
		return nil
	}
	return n
}
