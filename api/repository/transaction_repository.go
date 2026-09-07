package repository

import (
	"fmt"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ImplTransactionRepository(db *gorm.DB) contract.TransactionRepository {
	return &TransactionRepository{db: db}
}

type TransactionRepository struct {
	db *gorm.DB
}

func (r *TransactionRepository) CreateTransactionAndSyncHolding(payload *models.Transaction) (*models.Transaction, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var broker models.Broker
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", payload.BrokerID).First(&broker).Error; err != nil {
			return err
		}

		var stock models.StockHolding
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("ticker = ? AND broker_id = ?", payload.Ticker, payload.BrokerID).First(&stock).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		brokersFeeBuy := 0.0015
		brokersFeeSell := 0.0025

		grossAmount := float64(payload.Lot) * 100 * payload.Price
		buyFee := grossAmount * brokersFeeBuy
		sellFee := grossAmount * brokersFeeSell

		switch payload.Type {
		case models.TypeBuy:
			totalDebit := grossAmount + buyFee
			if broker.Cash < totalDebit {
				return fmt.Errorf("Insufficient broker cash: have Rp.%.1f, need Rp.%.1f", broker.Cash, totalDebit)
			}

			if err == gorm.ErrRecordNotFound {
				stock = models.StockHolding{
					Ticker:     payload.Ticker,
					Lot:        payload.Lot,
					AvgPrice:   payload.Price,
					BrokerID:   payload.BrokerID,
					BrokerName: payload.BrokerName,
				}
				if err := tx.Create(&stock).Error; err != nil {
					return err
				}
			} else {
				oldTotalValue := float64(stock.Lot) * 100 * stock.AvgPrice
				newTotalValue := float64(payload.Lot) * 100 * payload.Price
				newLot := stock.Lot + payload.Lot
				newAvg := (oldTotalValue + newTotalValue) / (float64(newLot) * 100)

				stock.Lot = newLot
				stock.AvgPrice = newAvg
				stock.BrokerID = payload.BrokerID
				stock.BrokerName = payload.BrokerName
				if err := tx.Save(&stock).Error; err != nil {
					return err
				}
			}

			// BUY: cash keluar = nilai transaksi + fee beli
			broker.Cash -= totalDebit

		case models.TypeSell:
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("cannot sell unknown stock %s", payload.Ticker)
			}
			if stock.Lot < payload.Lot {
				return fmt.Errorf("insufficient lot: have %d, want to sell %d", stock.Lot, payload.Lot)
			}

			stock.Lot -= payload.Lot
			stock.BrokerID = payload.BrokerID
			stock.BrokerName = payload.BrokerName

			if stock.Lot == 0 {
				if err := tx.Delete(&stock).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Save(&stock).Error; err != nil {
					return err
				}
			}

			// SELL: cash masuk = nilai transaksi - fee jual
			broker.Cash += (grossAmount - sellFee)

		default:
			return fmt.Errorf("invalid transaction type: %s", payload.Type)
		}

		if err := tx.Save(&broker).Error; err != nil {
			return err
		}

		payload.StockID = stock.Id
		if err := tx.Create(payload).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (r *TransactionRepository) GetTransactions(id int) ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := r.db.Where("id = ?", id).Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
} // For Development Purpose

func (r *TransactionRepository) GetAllTransactions() ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := r.db.Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *TransactionRepository) CountTransactionsByTicker(ticker string) (int64, error) {
	var count int64
	if err := r.db.Model(&models.Transaction{}).Where("ticker = ?", ticker).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *TransactionRepository) DeleteTransaction(id int) error {
	if err := r.db.Delete(&models.Transaction{}, id).Error; err != nil {
		return err
	}
	return nil
}
