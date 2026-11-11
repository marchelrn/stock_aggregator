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

func (r *StocksRepository) GetAllStocks(userID uint, limiter int) ([]models.StockHolding, error) {
	var stocks []models.StockHolding
	query := r.db.Preload("Broker").Where("user_id = ? AND lot > 0", userID)
	if limiter > 0 {
		query = query.Limit(limiter)
	}
	if err := query.Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *StocksRepository) GetStocks(userID uint, ticker []string) ([]models.StockHolding, error) {
	var stocks []models.StockHolding
	if err := r.db.Preload("Broker").Where("user_id = ? AND ticker IN ? AND lot > 0", userID, ticker).Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *StocksRepository) GetStocksReturnModels(userID uint, ticker []string) (*models.StockHolding, error) {
	var stock models.StockHolding
	if err := r.db.Preload("Broker").Where("user_id = ? AND ticker IN ? AND lot > 0", userID, ticker).First(&stock).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *StocksRepository) GetStockByTickerAndBroker(userID uint, ticker string, brokerID int) (*models.StockHolding, error) {
	var stock models.StockHolding
	if err := r.db.Preload("Broker").Where("user_id = ? AND ticker = ? AND broker_id = ?", userID, ticker, brokerID).First(&stock).Error; err != nil {
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

func (r *StocksRepository) UpdateStockData(userID uint, ticker []string, payload *models.StockHolding) (*models.StockHolding, error) {
	var stock models.StockHolding
	if err := r.db.Where("user_id = ? AND ticker IN ?", userID, ticker).First(&stock).Error; err != nil {
		return nil, err
	}

	stock.Ticker = payload.Ticker
	if payload.Sector != "" {
		stock.Sector = payload.Sector
	}
	stock.Lot = payload.Lot
	stock.AvgPrice = payload.AvgPrice
	stock.BrokerID = payload.BrokerID
	stock.BrokerName = payload.BrokerName

	if err := r.db.Save(&stock).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *StocksRepository) DeleteStockData(userID uint, stockTicker []string) error {
	if err := r.db.Where("user_id = ? AND ticker IN ?", userID, stockTicker).Delete(&models.StockHolding{}).Error; err != nil {
		return err
	}
	return nil
}

func (r *StocksRepository) GetStockById(userID uint, id int) (*models.StockHolding, error) {
	var stock models.StockHolding
	if err := r.db.Preload("Broker").Where("user_id = ? AND id = ?", userID, id).First(&stock).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *StocksRepository) UpdateStockSector(userID uint, ticker string, sector string) error {
	return r.db.Model(&models.StockHolding{}).Where("user_id = ? AND ticker = ?", userID, ticker).Update("sector", sector).Error
}

func (r *StocksRepository) UpdateSectorByTicker(ticker string, sector string) error {
	return r.db.Model(&models.StockHolding{}).Where("ticker = ?", ticker).Update("sector", sector).Error
}
