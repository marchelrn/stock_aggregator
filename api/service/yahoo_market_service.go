package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

const (
	marketCacheTTL = 5 * time.Minute
	marketCacheMax = 256
	marketBodyMax  = 2 << 20
)

var idxTicker = regexp.MustCompile(`^[A-Z0-9]{1,12}$`)

type cachedMarket struct {
	data      dto.MarketResponse
	fetchedAt time.Time
}

func (s *YahooService) GetMarket(ctx context.Context, ticker string) (*dto.MarketResponse, error) {
	ticker = strings.ToUpper(ticker)
	if !idxTicker.MatchString(ticker) {
		return nil, errs.BadRequest("ticker must contain 1 to 12 letters A-Z or digits 0-9 (without .JK)")
	}

	s.marketMu.Lock()
	cached, ok := s.marketCache[ticker]
	if ok && time.Since(cached.fetchedAt) < marketCacheTTL {
		data := cloneMarket(cached.data)
		s.marketMu.Unlock()
		return data, nil
	}
	s.marketMu.Unlock()

	data, err := s.fetchMarket(ctx, ticker)
	if err != nil {
		return nil, err
	}
	fetchedAt := time.Now().UTC()
	data.FetchedAt = fetchedAt.Format(time.RFC3339Nano)

	s.marketMu.Lock()
	defer s.marketMu.Unlock()
	if s.marketCache == nil {
		s.marketCache = make(map[string]cachedMarket)
	}
	for key, entry := range s.marketCache {
		if fetchedAt.Sub(entry.fetchedAt) >= marketCacheTTL {
			delete(s.marketCache, key)
		}
	}
	if _, exists := s.marketCache[ticker]; !exists && len(s.marketCache) >= marketCacheMax {
		var oldestKey string
		var oldest time.Time
		for key, entry := range s.marketCache {
			if oldestKey == "" || entry.fetchedAt.Before(oldest) {
				oldestKey, oldest = key, entry.fetchedAt
			}
		}
		delete(s.marketCache, oldestKey)
	}
	s.marketCache[ticker] = cachedMarket{data: *data, fetchedAt: fetchedAt}
	return cloneMarket(*data), nil
}

func (s *YahooService) fetchMarket(ctx context.Context, ticker string) (*dto.MarketResponse, error) {
	url := fmt.Sprintf("%s/v8/finance/chart/%s.JK?range=1mo&interval=1d", s.marketBaseURL, ticker)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, marketError(http.StatusBadGateway, "failed to create Yahoo chart request")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")
	resp, err := s.marketClient.Do(req)
	if err != nil {
		status := http.StatusBadGateway
		var timeout interface{ Timeout() bool }
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
			status = http.StatusGatewayTimeout
		}
		return nil, marketError(status, "failed to fetch Yahoo chart data")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		status := http.StatusBadGateway
		switch resp.StatusCode {
		case http.StatusNotFound, http.StatusTooManyRequests:
			status = resp.StatusCode
		}
		return nil, marketError(status, fmt.Sprintf("Yahoo chart returned HTTP %d", resp.StatusCode))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, marketBodyMax+1))
	if err != nil {
		status := http.StatusBadGateway
		var timeout interface{ Timeout() bool }
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
			status = http.StatusGatewayTimeout
		}
		return nil, marketError(status, "failed to read Yahoo chart response")
	}
	if len(body) > marketBodyMax {
		return nil, marketError(http.StatusBadGateway, "Yahoo chart response exceeds size limit")
	}
	var response dto.YahooMarketResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, marketError(http.StatusBadGateway, "invalid Yahoo chart response")
	}
	if chartErr := response.Chart.Error; chartErr != nil {
		status := http.StatusBadGateway
		switch strings.ToLower(chartErr.Code) {
		case "not found":
			status = http.StatusNotFound
		case "too many requests":
			status = http.StatusTooManyRequests
		}
		return nil, marketError(status, fmt.Sprintf("Yahoo chart error (%s): %s", chartErr.Code, chartErr.Description))
	}
	if len(response.Chart.Result) == 0 {
		return nil, marketError(http.StatusBadGateway, "Yahoo chart response contains no result")
	}
	result := response.Chart.Result[0]
	if result.Meta.Symbol != ticker+".JK" {
		return nil, marketError(http.StatusBadGateway, "Yahoo chart returned an unexpected symbol")
	}
	return parseMarket(ticker, result), nil
}

func parseMarket(ticker string, result dto.YahooMarketResult) *dto.MarketResponse {
	meta := result.Meta
	data := &dto.MarketResponse{
		Ticker: ticker, Name: meta.LongName, Currency: meta.Currency, Exchange: meta.FullExchangeName,
		Price: finite(meta.RegularMarketPrice), PreviousClose: finite(meta.PreviousClose),
		DayHigh: finite(meta.RegularMarketDayHigh), DayLow: finite(meta.RegularMarketDayLow),
		FiftyTwoWeekHigh: finite(meta.FiftyTwoWeekHigh), FiftyTwoWeekLow: finite(meta.FiftyTwoWeekLow),
		Volume: meta.RegularMarketVolume, MarketTime: meta.RegularMarketTime,
		History: make([]dto.MarketPoint, 0, len(result.Timestamp)),
	}
	if data.Name == "" {
		data.Name = meta.ShortName
	}
	if data.Exchange == "" {
		data.Exchange = meta.ExchangeName
	}
	if len(result.Indicators.Quote) > 0 {
		for i, close := range result.Indicators.Quote[0].Close {
			if i >= len(result.Timestamp) {
				break
			}
			if timestamp := result.Timestamp[i]; timestamp != nil && *timestamp > 0 && finite(close) != nil {
				data.History = append(data.History, dto.MarketPoint{Time: *timestamp, Close: *close})
			}
		}
	}
	sort.Slice(data.History, func(i, j int) bool { return data.History[i].Time < data.History[j].Time })
	// chartPreviousClose is the start-of-range close, not yesterday's close.
	// When previousClose is absent, use the last valid daily close before the
	// quote's exchange-local calendar date, never the current day's partial bar.
	if data.PreviousClose == nil && data.MarketTime != nil {
		location, err := time.LoadLocation(meta.ExchangeTimezoneName)
		if err != nil {
			location = time.FixedZone("exchange", meta.GMTOffset)
		}
		quoteTime := time.Unix(*data.MarketTime, 0).In(location)
		dayStart := time.Date(quoteTime.Year(), quoteTime.Month(), quoteTime.Day(), 0, 0, 0, 0, location).Unix()
		for _, point := range data.History {
			if point.Time < dayStart {
				close := point.Close
				data.PreviousClose = &close
			}
		}
	}
	if data.Price != nil && data.PreviousClose != nil {
		change := *data.Price - *data.PreviousClose
		data.Change = finite(&change)
		if *data.PreviousClose != 0 && data.Change != nil {
			percent := change / *data.PreviousClose * 100
			data.ChangePercent = finite(&percent)
		}
	}
	return data
}

func finite(value *float64) *float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return nil
	}
	return value
}

func marketError(status int, message string) error {
	return &errs.AppError{Code: status, Message: message}
}

// Return detached data so callers cannot mutate cached pointers or slices.
func cloneMarket(data dto.MarketResponse) *dto.MarketResponse {
	copyFloat := func(value *float64) *float64 {
		if value == nil {
			return nil
		}
		copy := *value
		return &copy
	}
	copyInt := func(value *int64) *int64 {
		if value == nil {
			return nil
		}
		copy := *value
		return &copy
	}
	data.Price = copyFloat(data.Price)
	data.PreviousClose = copyFloat(data.PreviousClose)
	data.Change = copyFloat(data.Change)
	data.ChangePercent = copyFloat(data.ChangePercent)
	data.DayHigh = copyFloat(data.DayHigh)
	data.DayLow = copyFloat(data.DayLow)
	data.FiftyTwoWeekHigh = copyFloat(data.FiftyTwoWeekHigh)
	data.FiftyTwoWeekLow = copyFloat(data.FiftyTwoWeekLow)
	data.Volume = copyInt(data.Volume)
	data.MarketTime = copyInt(data.MarketTime)
	data.History = append([]dto.MarketPoint{}, data.History...)
	return &data
}
