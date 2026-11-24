package repository

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range mutations {
			if item == nil || item.Date.IsZero() {
				return fmt.Errorf("cash mutation date is required for cash eligibility")
			}
			item.UserID = userID
			item.AppliedDelta = nil
			if item.Amount <= 0 || math.IsNaN(item.Amount) || math.IsInf(item.Amount, 0) || MutationCashDelta(item) == 0 {
				return fmt.Errorf("cash mutation requires a supported type and a positive finite amount")
			}

			if item.BrokerID == 0 && item.BrokerName != "" {
				var broker models.Broker
				err := tx.Where("user_id = ? AND LOWER(name) = LOWER(?)", userID, strings.TrimSpace(item.BrokerName)).First(&broker).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					broker = models.Broker{UserID: userID, Name: strings.TrimSpace(item.BrokerName)}
					_, err = ImplBrokersRepository(tx).AddBrokerData(&broker)
				}
				if err != nil {
					return err
				}
				item.BrokerID = broker.ID
			}

			// Serialize deduplication with the balance update, including retries.
			var broker models.Broker
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, item.BrokerID).First(&broker).Error; err != nil {
				return err
			}
			if err := prepareCashBaseline(tx, userID, &broker); err != nil {
				return err
			}
			applyCash := CountsAfterCutoff(CashEventTime(item), broker.CashBaseAsOf)

			var existing models.CashMutation
			if item.RefNo != "" {
				err := tx.Where("user_id = ? AND broker_id = ? AND ref_no = ?", userID, item.BrokerID, item.RefNo).First(&existing).Error
				if err == nil {
					continue
				}
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}

			start := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, item.Date.Location())
			legacy := tx.Where("user_id = ? AND broker_id = ? AND type = ? AND amount = ? AND date >= ? AND date < ? AND COALESCE(ref_no, '') = ''",
				userID, item.BrokerID, item.Type, item.Amount, start, start.AddDate(0, 0, 1))
			if item.EmailReceivedAt != nil {
				legacy = legacy.Where("email_received_at IS NULL OR email_received_at = ?", *item.EmailReceivedAt)
			}
			err := legacy.First(&existing).Error
			if err == nil {
				if item.RefNo != "" {
					if err := tx.Model(&existing).Update("ref_no", item.RefNo).Error; err != nil {
						return err
					}
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			if applyCash && item.Type == models.CashMutationWithdraw && item.Amount > broker.Cash {
				delta := -broker.Cash
				item.AppliedDelta = &delta
			}
			if err := tx.Create(item).Error; err != nil {
				return err
			}
			syncedCount++
			if !applyCash {
				continue
			}
			broker.Cash += MutationCashDelta(item)
			if err := tx.Save(&broker).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return syncedCount, nil
}
