package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type YahooService struct {
	YahooRepository contract.YahooRepository
	marketClient    *http.Client
	marketBaseURL   string
	marketMu        sync.Mutex
	marketCache     map[string]cachedMarket
}

func ImplYahooService(repo contract.YahooRepository) contract.YahooService {
	return &YahooService{
		YahooRepository: repo,
		marketClient:    &http.Client{Timeout: 10 * time.Second},
		marketBaseURL:   "https://query1.finance.yahoo.com",
		marketCache:     make(map[string]cachedMarket),
	}
}

func (s *YahooService) GetPrice(ticker string) (*models.StockPrice, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	ticker = strings.TrimSuffix(ticker, ".JK")
	if ticker == "" {
		return nil, errs.BadRequest("ticker is required")
	}

	if s.YahooRepository != nil {
		if cached, err := s.YahooRepository.GetPrice(ticker); err == nil && cached != nil {
			if time.Since(cached.UpdatedAt) < 5*time.Minute {
				if cached.Sector == "" {
					if sec, err := s.GetSector(ticker); err == nil && sec != "" {
						cached.Sector = sec
						_ = s.YahooRepository.UpdateSector(ticker, sec)
					}
				}
				return cached, nil
			}
		}
	}

	baseURL := s.marketBaseURL
	if baseURL == "" {
		baseURL = "https://query1.finance.yahoo.com"
	}
	url := fmt.Sprintf("%s/v8/finance/chart/%s.JK", baseURL, ticker)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	client := s.marketClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
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

	sector, _ := s.GetSector(ticker)

	price := &models.StockPrice{
		Ticker:        ticker,
		Sector:        sector,
		Price:         meta.RegularMarketPrice,
		PreviousClose: meta.PreviousClose,
		Change:        change,
		ChangePercent: changePct,
		Currency:      meta.Currency,
	}

	price.UpdatedAt = time.Now()
	// Update cache in database over memory cache
	if s.YahooRepository != nil {
		if err := s.YahooRepository.SavePrice(price); err != nil {
			// Just log or ignore since we can still return the fetched price
			fmt.Printf("Failed to cache price for %s: %v\n", ticker, err)
		}
	}

	return price, nil
}

func (s *YahooService) GetAllPrices() ([]models.StockPrice, error) {
	return s.YahooRepository.GetAllPrices()
}

// GetPrices mengambil harga untuk banyak ticker sekaligus.
//
// Mengembalikan:
//   - prices: ticker yang berhasil diambil
//   - failed: ticker yang gagal beserta alasannya (ticker -> pesan error)
//   - err:    hanya terisi jika SEMUA ticker gagal
//
// Dengan begitu pemanggil bisa membedakan hasil sebagian (partial) dari hasil penuh,
// bukan menganggap seluruh permintaan berhasil hanya karena sebagian ticker valid.
func (s *YahooService) GetPrices(tickers []string) (map[string]*models.StockPrice, map[string]string, error) {
	prices := make(map[string]*models.StockPrice)
	failed := make(map[string]string)

	// Normalisasi: trim, buang yang kosong, dan hilangkan duplikat.
	seen := make(map[string]struct{}, len(tickers))
	unique := make([]string, 0, len(tickers))
	for _, t := range tickers {
		t = strings.ToUpper(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		unique = append(unique, t)
	}

	if len(unique) == 0 {
		return nil, nil, errs.BadRequest("ticker is required")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, ticker := range unique {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			price, err := s.GetPrice(t)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed[t] = err.Error()
				return
			}
			prices[t] = price
		}(ticker)
	}

	wg.Wait()

	if len(prices) == 0 {
		names := make([]string, 0, len(failed))
		for t := range failed {
			names = append(names, t)
		}
		sort.Strings(names)
		return nil, failed, errs.BadRequest(fmt.Sprintf("yahoo API error: no data found for %s", strings.Join(names, ", ")))
	}

	return prices, failed, nil
}

func (s *YahooService) GetSector(ticker string) (string, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	ticker = strings.TrimSuffix(ticker, ".JK")
	if ticker == "" {
		return "", errs.BadRequest("ticker is required")
	}

	// 1. Check database cache
	if s.YahooRepository != nil {
		if cached, err := s.YahooRepository.GetPrice(ticker); err == nil && cached != nil && cached.Sector != "" {
			return cached.Sector, nil
		}
	}

	// 2. Fetch from Yahoo Finance Search API
	baseURL := s.marketBaseURL
	if baseURL == "" {
		baseURL = "https://query1.finance.yahoo.com"
	}
	url := fmt.Sprintf("%s/v1/finance/search?q=%s.JK", baseURL, ticker)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	client := s.marketClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to search sector for %s: %w", ticker, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch sector for %s, status code: %d", ticker, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var searchRes struct {
		Quotes []struct {
			Symbol     string `json:"symbol"`
			Sector     string `json:"sector"`
			SectorDisp string `json:"sectorDisp"`
		} `json:"quotes"`
	}

	if err := json.Unmarshal(body, &searchRes); err != nil {
		return "", fmt.Errorf("failed to parse search response: %w", err)
	}

	targetJK := ticker + ".JK"
	var sector string
	for _, q := range searchRes.Quotes {
		if strings.EqualFold(q.Symbol, targetJK) || strings.EqualFold(q.Symbol, ticker) || strings.EqualFold(strings.TrimSuffix(q.Symbol, ".JK"), ticker) {
			sector = q.SectorDisp
			if sector == "" {
				sector = q.Sector
			}
			break
		}
	}

	if sector != "" && s.YahooRepository != nil {
		_ = s.YahooRepository.UpdateSector(ticker, sector)
	}

	return sector, nil
}

func (s *YahooService) GetSectors(tickers []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(tickers) == 0 {
		return result, nil
	}

	seen := make(map[string]struct{}, len(tickers))
	unique := make([]string, 0, len(tickers))
	for _, t := range tickers {
		t = strings.ToUpper(strings.TrimSpace(t))
		t = strings.TrimSuffix(t, ".JK")
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		unique = append(unique, t)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, ticker := range unique {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			sector, err := s.GetSector(t)
			if err == nil && sector != "" {
				mu.Lock()
				result[t] = sector
				mu.Unlock()
			}
		}(ticker)
	}

	wg.Wait()
	return result, nil
}
