package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
)

type CashMutationRepository struct {
	db *gorm.DB
}

func ImplCashMutationRepository(db *gorm.DB) *CashMutationRepository {
	return &CashMutationRepository{db: db}
}

func (r *CashMutationRepository) GetAll(userID uint) ([]models.CashMutation, error) {
	var mutations []models.CashMutation
	err := r.db.Where("user_id = ?", userID).Order("date DESC, id DESC").Find(&mutations).Error
	if err != nil {
		return nil, err
	}
	return mutations, nil
}

func (r *CashMutationRepository) SyncCashMutationBatch(userID uint, mutations []*models.CashMutation) (int, error) {
	if len(mutations) == 0 {
		return 0, nil
	}

	syncedCount := 0

	for _, item := range mutations {
		item.UserID = userID
		if item.Date.IsZero() {
			item.Date = time.Now()
		}

		// Resolve broker
		if item.BrokerID == 0 && item.BrokerName != "" {
			var b models.Broker
			if err := r.db.Where("user_id = ? AND LOWER(name) = LOWER(?)", userID, strings.TrimSpace(item.BrokerName)).First(&b).Error; err == nil {
				item.BrokerID = b.ID
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				newBroker := models.Broker{
					UserID: userID,
					Name:   strings.TrimSpace(item.BrokerName),
					Cash:   0,
				}
				if createErr := r.db.Create(&newBroker).Error; createErr == nil {
					item.BrokerID = newBroker.ID
				}
			}
		}

		// Check duplicate
		var existing models.CashMutation
		var isDuplicate bool

		if item.RefNo != "" {
			err := r.db.Where("user_id = ? AND broker_id = ? AND ref_no = ?", userID, item.BrokerID, item.RefNo).First(&existing).Error
			if err == nil {
				isDuplicate = true
			}
		}

		if !isDuplicate {
			// Check by user, broker, type, amount, and date (same day)
			startOfDay := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, item.Date.Location())
			endOfDay := startOfDay.Add(24 * time.Hour)
			err := r.db.Where("user_id = ? AND broker_id = ? AND type = ? AND amount = ? AND date >= ? AND date < ?",
				userID, item.BrokerID, item.Type, item.Amount, startOfDay, endOfDay).First(&existing).Error
			if err == nil {
				isDuplicate = true
			}
		}

		if isDuplicate {
			continue
		}

		// Insert mutation and update broker balance in a single transaction
		txErr := r.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(item).Error; err != nil {
				return err
			}

			// Update broker cash
			var cashDiff float64
			if item.Type == models.CashMutationDeposit {
				cashDiff = item.Amount
			} else if item.Type == models.CashMutationWithdraw {
				cashDiff = -item.Amount
			}

			if cashDiff != 0 && item.BrokerID > 0 {
				if err := tx.Model(&models.Broker{}).
					Where("id = ? AND user_id = ?", item.BrokerID, userID).
					Update("cash", gorm.Expr("cash + ?", cashDiff)).Error; err != nil {
					return err
				}
			}

			return nil
		})

		if txErr == nil {
			syncedCount++
		}
	}

	return syncedCount, nil
}
