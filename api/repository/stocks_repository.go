package repository

import (
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/models"

	"gorm.io/gorm"
)

type StocksRepository struct {
	db *gorm.DB
}

func ImplStocksRepository(db *gorm.DB) contract.StocksRepository {
	return &StocksRepository{db: db}
}

func (r *StocksRepository) GetAllStocks(limiter int) ([]models.StockHolding, error) {
	var stocks []models.StockHolding
	if err := r.db.Preload("Broker").Limit(limiter).Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *StocksRepository) GetStocks(ticker []string) ([]models.StockHolding, error) {
	var stocks []models.StockHolding
	if err := r.db.Preload("Broker").Where("ticker IN ?", ticker).Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *StocksRepository) GetStocksReturnModels(ticker []string) (*models.StockHolding, error) {
	var stock models.StockHolding
	if err := r.db.Preload("Broker").Where("ticker IN ?", ticker).First(&stock).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *StocksRepository) GetStockByTickerAndBroker(ticker string, brokerID int) (*models.StockHolding, error) {
	var stock models.StockHolding
	if err := r.db.Preload("Broker").Where("ticker = ? AND broker_id = ?", ticker, brokerID).First(&stock).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *StocksRepository) AddStockData(payload *models.StockHolding) (*models.StockHolding, error) {
	if err := r.db.Create(payload).Error; err != nil {
		return nil, err
	}
	return payload, nil
}

func (r *StocksRepository) UpdateStockData(ticker []string, payload *models.StockHolding) (*models.StockHolding, error) {
	var stock models.StockHolding
	if err := r.db.Where("ticker IN ?", ticker).First(&stock).Error; err != nil {
		return nil, err
	}

	stock.Ticker = payload.Ticker
	stock.Lot = payload.Lot
	stock.AvgPrice = payload.AvgPrice
	stock.BrokerID = payload.BrokerID
	stock.BrokerName = payload.BrokerName

	if err := r.db.Save(&stock).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *StocksRepository) DeleteStockData(stockTicker []string) error {
	if err := r.db.Where("ticker IN ?", stockTicker).Delete(&models.StockHolding{}).Error; err != nil {
		return err
	}
	return nil
}

func (r *StocksRepository) GetStockById(id int) (*models.StockHolding, error) {
	var stock models.StockHolding
	if err := r.db.Preload("Broker").Where("id = ?", id).First(&stock).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}
