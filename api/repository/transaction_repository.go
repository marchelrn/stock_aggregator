package repository

import (
	"fmt"
	"log"
	"strings"

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
		if payload.BrokerID == 0 && payload.BrokerName != "" {
			var b models.Broker
			if err := tx.Where("user_id = ? AND LOWER(name) = LOWER(?)", payload.UserID, strings.TrimSpace(payload.BrokerName)).First(&b).Error; err == nil {
				payload.BrokerID = b.ID
			}
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", payload.UserID, payload.BrokerID).First(&broker).Error; err != nil {
			return err
		}

		var existingCount int64
		tx.Model(&models.Transaction{}).
			Where("user_id = ? AND ticker = ? AND type = ? AND lot = ? AND price = ? AND DATE(date) = DATE(?) AND broker_id = ?",
				payload.UserID, payload.Ticker, payload.Type, payload.Lot, payload.Price, payload.Date, payload.BrokerID).
			Count(&existingCount)
		if existingCount > 0 {
			log.Printf("Transaction %s %s %d lot @ %.2f on %s already exists, skipping duplicate",
				payload.Type, payload.Ticker, payload.Lot, payload.Price, payload.Date.Format("2006-01-02"))
			return nil
		}

		var stock models.StockHolding
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND ticker = ? AND broker_id = ?", payload.UserID, payload.Ticker, payload.BrokerID).First(&stock).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		grossAmount := float64(payload.Lot) * 100 * payload.Price

		// Use the actual tax from the parsed trade confirmation if available,
		// otherwise fallback to default broker fee rates
		tax := payload.Tax
		if tax == 0 {
			switch payload.Type {
			case models.TypeBuy:
				tax = grossAmount * 0.0015
			case models.TypeSell:
				tax = grossAmount * 0.0025
			}
		}

		// amountDone = gross + tax (total cost including fees)
		amountDone := grossAmount + tax

		switch payload.Type {
		case models.TypeBuy:
			if broker.Cash < amountDone {
				return fmt.Errorf("Insufficient broker cash: have Rp.%.1f, need Rp.%.1f", broker.Cash, amountDone)
			}

			// avg_price includes tax: amountDone / shares
			shares := float64(payload.Lot) * 100
			avgPriceWithTax := amountDone / shares

			if err == gorm.ErrRecordNotFound {
				stock = models.StockHolding{
					UserID:     payload.UserID,
					Ticker:     payload.Ticker,
					Lot:        payload.Lot,
					AvgPrice:   avgPriceWithTax,
					BrokerID:   payload.BrokerID,
					BrokerName: payload.BrokerName,
				}
				if err := tx.Create(&stock).Error; err != nil {
					return err
				}
			} else {
				oldTotalValue := float64(stock.Lot) * 100 * stock.AvgPrice
				newTotalValue := amountDone
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

			// BUY: cash keluar = gross + tax
			broker.Cash -= amountDone

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

			if err := tx.Save(&stock).Error; err != nil {
				return err
			}

			// SELL: cash masuk = gross - tax
			broker.Cash += (grossAmount - tax)

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

func (r *TransactionRepository) GetTransactions(userID uint, id int) ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := r.db.Where("user_id = ? AND id = ?", userID, id).Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *TransactionRepository) GetAllTransactions(userID uint) ([]models.Transaction, error) {
	var transactions []models.Transaction
	if err := r.db.Where("user_id = ?", userID).Order("date DESC, id DESC").Find(&transactions).Error; err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *TransactionRepository) CountTransactionsByTicker(userID uint, ticker string) (int64, error) {
	var count int64
	if err := r.db.Model(&models.Transaction{}).Where("user_id = ? AND ticker = ?", userID, ticker).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *TransactionRepository) DeleteTransaction(userID uint, id int) error {
	result := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&models.Transaction{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
