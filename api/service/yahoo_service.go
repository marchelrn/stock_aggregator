package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type YahooService struct {
	YahooRepository contract.YahooRepository
}

func ImplYahooService(repo contract.YahooRepository) contract.YahooService {
	return &YahooService{
		YahooRepository: repo,
	}
}

func (s *YahooService) GetPrice(ticker string) (*models.StockPrice, error) {
	if cached, err := s.YahooRepository.GetPrice(ticker); err == nil {
		if time.Since(cached.UpdatedAt) < 5*time.Minute {
			return cached, nil
		}
	}

	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s.JK", ticker)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch price for %s: %w", ticker, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var data dto.YahooResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if data.Chart.Error != nil {
		return nil, errs.BadRequest(fmt.Sprintf("yahoo API error: %s", data.Chart.Error.Description))
	}

	if len(data.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data found for ticker: %s", ticker)
	}

	meta := data.Chart.Result[0].Meta
	change := meta.RegularMarketPrice - meta.PreviousClose
	changePct := 0.0
	if meta.PreviousClose > 0 {
		changePct = (change / meta.PreviousClose) * 100
	}

	price := &models.StockPrice{
		Ticker:        ticker,
		Price:         meta.RegularMarketPrice,
		PreviousClose: meta.PreviousClose,
		Change:        change,
		ChangePercent: changePct,
		Currency:      meta.Currency,
	}

	price.UpdatedAt = time.Now()
	// Update cache in database over memory cache
	if err := s.YahooRepository.SavePrice(price); err != nil {
		// Just log or ignore since we can still return the fetched price
		fmt.Printf("Failed to cache price for %s: %v\n", ticker, err)
	}

	return price, nil
}

func (s *YahooService) GetAllPrices() ([]models.StockPrice, error) {
	return s.YahooRepository.GetAllPrices()
}

func (s *YahooService) GetPrices(tickers []string) (map[string]*models.StockPrice, error) {
	prices := make(map[string]*models.StockPrice)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, ticker := range tickers {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			price, err := s.GetPrice(t)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = errs.BadRequest("yahoo API error: No data found, symbol may be delisted")
				}
				return
			}
			prices[t] = price
		}(ticker)
	}

	wg.Wait()

	if len(prices) == 0 && firstErr != nil {
		return nil, firstErr
	}

	return prices, nil
}
