package dto

// MarketResponse keeps unavailable numeric values distinct from actual zeroes.
type MarketResponse struct {
	Ticker           string        `json:"ticker"`
	Name             string        `json:"name"`
	Currency         string        `json:"currency"`
	Exchange         string        `json:"exchange"`
	Price            *float64      `json:"price"`
	PreviousClose    *float64      `json:"previous_close"`
	Change           *float64      `json:"change"`
	ChangePercent    *float64      `json:"change_percent"`
	DayHigh          *float64      `json:"day_high"`
	DayLow           *float64      `json:"day_low"`
	FiftyTwoWeekHigh *float64      `json:"fifty_two_week_high"`
	FiftyTwoWeekLow  *float64      `json:"fifty_two_week_low"`
	Volume           *int64        `json:"volume"`
	MarketTime       *int64        `json:"market_time"`
	FetchedAt        string        `json:"fetched_at"`
	History          []MarketPoint `json:"history"`
}

type MarketPoint struct {
	Time  int64   `json:"time"`
	Close float64 `json:"close"`
}

// These chart structures are separate from the legacy price DTOs so null handling
// and history support do not change the existing price endpoints.
type YahooMarketResponse struct {
	Chart struct {
		Result []YahooMarketResult `json:"result"`
		Error  *Error              `json:"error"`
	} `json:"chart"`
}

type YahooMarketResult struct {
	Meta       YahooMarketMeta `json:"meta"`
	Timestamp  []*int64        `json:"timestamp"`
	Indicators struct {
		Quote []struct {
			Close []*float64 `json:"close"`
		} `json:"quote"`
	} `json:"indicators"`
}

type YahooMarketMeta struct {
	Symbol               string   `json:"symbol"`
	LongName             string   `json:"longName"`
	ShortName            string   `json:"shortName"`
	Currency             string   `json:"currency"`
	ExchangeName         string   `json:"exchangeName"`
	FullExchangeName     string   `json:"fullExchangeName"`
	ExchangeTimezoneName string   `json:"exchangeTimezoneName"`
	GMTOffset            int      `json:"gmtoffset"`
	RegularMarketPrice   *float64 `json:"regularMarketPrice"`
	PreviousClose        *float64 `json:"previousClose"`
	RegularMarketTime    *int64   `json:"regularMarketTime"`
	RegularMarketDayHigh *float64 `json:"regularMarketDayHigh"`
	RegularMarketDayLow  *float64 `json:"regularMarketDayLow"`
	FiftyTwoWeekHigh     *float64 `json:"fiftyTwoWeekHigh"`
	FiftyTwoWeekLow      *float64 `json:"fiftyTwoWeekLow"`
	RegularMarketVolume  *int64   `json:"regularMarketVolume"`
}
