package repository

import (
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type YahooRepository struct {
	db *gorm.DB
}

func ImplYahooRepository(db *gorm.DB) contract.YahooRepository {
	return &YahooRepository{db: db}
}

func (r *YahooRepository) GetPrice(ticker string) (*models.StockPrice, error) {
	var price models.StockPrice
	err := r.db.Where("ticker = ?", ticker).First(&price).Error
	if err != nil {
		return nil, err
	}
	return &price, nil
}

func (r *YahooRepository) GetPrices(tickers []string) (map[string]*models.StockPrice, error) {
	var prices []models.StockPrice
	err := r.db.Where("ticker IN ?", tickers).Find(&prices).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]*models.StockPrice)
	for i := range prices {
		result[prices[i].Ticker] = &prices[i]
	}
	return result, nil
}

func (r *YahooRepository) GetAllPrices() ([]models.StockPrice, error) {
	var prices []models.StockPrice
	err := r.db.Order("ticker ASC").Find(&prices).Error
	if err != nil {
		return nil, err
	}
	return prices, nil
}

func (r *YahooRepository) SavePrice(price *models.StockPrice) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "ticker"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"price", "previous_close", "change", "change_percent", "currency", "updated_at",
		}),
	}).Create(price).Error
}
