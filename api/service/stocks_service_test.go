package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
)

type mockStocksRepo struct {
	stocks []models.StockHolding
}

func (m *mockStocksRepo) GetAllStocks(userID uint, limiter int) ([]models.StockHolding, error) {
	var res []models.StockHolding
	for _, s := range m.stocks {
		if s.UserID == userID && s.Lot > 0 {
			res = append(res, s)
		}
	}
	return res, nil
}

func (m *mockStocksRepo) GetStocks(userID uint, ticker []string) ([]models.StockHolding, error) {
	var res []models.StockHolding
	tickerMap := make(map[string]bool)
	for _, t := range ticker {
		tickerMap[t] = true
	}
	for _, s := range m.stocks {
		if s.UserID == userID && tickerMap[s.Ticker] && s.Lot > 0 {
			res = append(res, s)
		}
	}
	return res, nil
}

func (m *mockStocksRepo) GetStocksReturnModels(userID uint, ticker []string) (*models.StockHolding, error) {
	stocks, err := m.GetStocks(userID, ticker)
	if err != nil || len(stocks) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &stocks[0], nil
}

func (m *mockStocksRepo) GetStockByTickerAndBroker(userID uint, ticker string, brokerID int) (*models.StockHolding, error) {
	for i := range m.stocks {
		if m.stocks[i].UserID == userID && m.stocks[i].Ticker == ticker && m.stocks[i].BrokerID == brokerID {
			return &m.stocks[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockStocksRepo) GetStockById(userID uint, id int) (*models.StockHolding, error) {
	for i := range m.stocks {
		if m.stocks[i].UserID == userID && m.stocks[i].Id == id {
			return &m.stocks[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockStocksRepo) AddStockData(payload *models.StockHolding) (*models.StockHolding, error) {
	payload.Id = len(m.stocks) + 1
	m.stocks = append(m.stocks, *payload)
	return payload, nil
}

func (m *mockStocksRepo) UpdateStockData(userID uint, ticker []string, payload *models.StockHolding) (*models.StockHolding, error) {
	for i := range m.stocks {
		if m.stocks[i].UserID == userID && m.stocks[i].Ticker == payload.Ticker {
			if payload.Sector != "" {
				m.stocks[i].Sector = payload.Sector
			}
			m.stocks[i].Lot = payload.Lot
			m.stocks[i].AvgPrice = payload.AvgPrice
			m.stocks[i].BrokerID = payload.BrokerID
			m.stocks[i].BrokerName = payload.BrokerName
			m.stocks[i].UpdatedAt = time.Now()
			return &m.stocks[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockStocksRepo) DeleteStockData(userID uint, stockTicker []string) error {
	var filtered []models.StockHolding
	toDelete := make(map[string]bool)
	for _, t := range stockTicker {
		toDelete[t] = true
	}
	for _, s := range m.stocks {
		if !(s.UserID == userID && toDelete[s.Ticker]) {
			filtered = append(filtered, s)
		}
	}
	m.stocks = filtered
	return nil
}

func (m *mockStocksRepo) UpdateStockSector(userID uint, ticker string, sector string) error {
	for i := range m.stocks {
		if m.stocks[i].UserID == userID && m.stocks[i].Ticker == ticker {
			m.stocks[i].Sector = sector
		}
	}
	return nil
}

func (m *mockStocksRepo) UpdateSectorByTicker(ticker string, sector string) error {
	for i := range m.stocks {
		if m.stocks[i].Ticker == ticker {
			m.stocks[i].Sector = sector
		}
	}
	return nil
}

func TestGetAllStocksHidesZeroLot(t *testing.T) {
	repo := &mockStocksRepo{
		stocks: []models.StockHolding{
			{Id: 1, UserID: 1, Ticker: "BBCA", Lot: 100, AvgPrice: 9000, BrokerID: 1},
			{Id: 2, UserID: 1, Ticker: "BBRI", Lot: 0, AvgPrice: 0, BrokerID: 1}, // Sold out
			{Id: 3, UserID: 1, Ticker: "TLKM", Lot: 50, AvgPrice: 2800, BrokerID: 1},
		},
	}
	svc := &StocksService{StocksRepository: repo}

	res, err := svc.GetAllStocks(1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.StocksData) != 2 {
		t.Fatalf("expected 2 active holdings, got %d", len(res.StocksData))
	}
	for _, s := range res.StocksData {
		if s.Ticker == "BBRI" {
			t.Fatalf("zero lot stock BBRI should not be returned in GetAllStocks")
		}
	}
}

func TestAddStockDataReactivatesZeroLotStock(t *testing.T) {
	repo := &mockStocksRepo{
		stocks: []models.StockHolding{
			{Id: 1, UserID: 1, Ticker: "BBCA", Lot: 0, AvgPrice: 0, BrokerID: 1, BrokerName: "Stockbit"},
		},
	}
	svc := &StocksService{StocksRepository: repo}

	req := &dto.StocksRequest{
		Ticker:     "BBCA",
		Lot:        50,
		AvgPrice:   9200,
		BrokerID:   1,
		BrokerName: "Stockbit",
	}

	res, err := svc.AddStockData(1, req)
	if err != nil {
		t.Fatalf("unexpected error reactivating stock: %v", err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected StatusCreated, got %d", res.StatusCode)
	}
	if len(res.StocksData) != 1 {
		t.Fatalf("expected 1 stock returned, got %d", len(res.StocksData))
	}
	if res.StocksData[0].Lot != 50 || res.StocksData[0].AvgPrice != 9200 {
		t.Fatalf("expected lot=50, avg=9200, got lot=%d, avg=%.2f", res.StocksData[0].Lot, res.StocksData[0].AvgPrice)
	}
}

type mockYahooService struct {
	sectors map[string]string
}

func (m *mockYahooService) GetMarket(ctx context.Context, ticker string) (*dto.MarketResponse, error) {
	return nil, nil
}
func (m *mockYahooService) GetPrice(ticker string) (*models.StockPrice, error) {
	return nil, nil
}
func (m *mockYahooService) GetPrices(tickers []string) (map[string]*models.StockPrice, map[string]string, error) {
	return nil, nil, nil
}
func (m *mockYahooService) GetAllPrices() ([]models.StockPrice, error) {
	return nil, nil
}
func (m *mockYahooService) GetSector(ticker string) (string, error) {
	if m.sectors != nil {
		return m.sectors[ticker], nil
	}
	return "", nil
}
func (m *mockYahooService) GetSectors(tickers []string) (map[string]string, error) {
	res := make(map[string]string)
	if m.sectors != nil {
		for _, t := range tickers {
			if s, ok := m.sectors[t]; ok {
				res[t] = s
			}
		}
	}
	return res, nil
}

func TestGetAllStocksPopulatesMissingSectors(t *testing.T) {
	repo := &mockStocksRepo{
		stocks: []models.StockHolding{
			{Id: 1, UserID: 1, Ticker: "BBCA", Sector: "", Lot: 10, AvgPrice: 9000, BrokerID: 1},
			{Id: 2, UserID: 1, Ticker: "TLKM", Sector: "Communication Services", Lot: 20, AvgPrice: 3000, BrokerID: 1},
		},
	}
	yahoo := &mockYahooService{
		sectors: map[string]string{
			"BBCA": "Financial Services",
		},
	}
	svc := &StocksService{
		StocksRepository: repo,
		YahooService:     yahoo,
	}

	res, err := svc.GetAllStocks(1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.StocksData) != 2 {
		t.Fatalf("expected 2 stocks, got %d", len(res.StocksData))
	}

	for _, s := range res.StocksData {
		if s.Ticker == "BBCA" && s.Sector != "Financial Services" {
			t.Errorf("expected BBCA sector to be Financial Services, got %s", s.Sector)
		}
		if s.Ticker == "TLKM" && s.Sector != "Communication Services" {
			t.Errorf("expected TLKM sector to be Communication Services, got %s", s.Sector)
		}
	}

	// Verify sector was updated in repository
	for _, s := range repo.stocks {
		if s.Ticker == "BBCA" && s.Sector != "Financial Services" {
			t.Errorf("expected repo stock BBCA sector to be updated to Financial Services, got %s", s.Sector)
		}
	}
}

func TestSyncStockSectors(t *testing.T) {
	repo := &mockStocksRepo{
		stocks: []models.StockHolding{
			{Id: 1, UserID: 1, Ticker: "BBRI", Sector: "", Lot: 15, AvgPrice: 4200, BrokerID: 1},
		},
	}
	yahoo := &mockYahooService{
		sectors: map[string]string{
			"BBRI": "Financial Services",
		},
	}
	svc := &StocksService{
		StocksRepository: repo,
		YahooService:     yahoo,
	}

	res, err := svc.SyncStockSectors(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.StocksData) != 1 {
		t.Fatalf("expected 1 stock, got %d", len(res.StocksData))
	}
	if res.StocksData[0].Sector != "Financial Services" {
		t.Errorf("expected BBRI sector to be Financial Services, got %s", res.StocksData[0].Sector)
	}
}

func TestUpdateStockPreservesAndUpdatesSector(t *testing.T) {
	repo := &mockStocksRepo{
		stocks: []models.StockHolding{
			{Id: 1, UserID: 1, Ticker: "BBCA", Sector: "Financial Services", Lot: 10, AvgPrice: 9000, BrokerID: 1},
		},
	}
	svc := &StocksService{StocksRepository: repo}

	// Update lot without touching sector
	res, err := svc.UpdateStock(1, []string{"BBCA"}, &dto.StocksRequest{Lot: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.StocksData) != 1 || res.StocksData[0].Sector != "Financial Services" {
		t.Errorf("expected sector to be preserved as Financial Services, got %s", res.StocksData[0].Sector)
	}

	// Update sector explicitly
	res2, err := svc.UpdateStock(1, []string{"BBCA"}, &dto.StocksRequest{Sector: "Banking"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res2.StocksData) != 1 || res2.StocksData[0].Sector != "Banking" {
		t.Errorf("expected sector to be updated to Banking, got %s", res2.StocksData[0].Sector)
	}
}
