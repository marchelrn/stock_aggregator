package repository

import (
	"math"

	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *BrokersRepository) TransferCash(userID uint, fromID, toID int, amount float64) error {
	if fromID <= 0 || toID <= 0 || fromID == toID || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return errs.BadRequest("Transfer requires distinct brokers and a positive finite amount")
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Stable lock order prevents opposite-direction transfers deadlocking.
		ids := []int{fromID, toID}
		if ids[0] > ids[1] {
			ids[0], ids[1] = ids[1], ids[0]
		}
		brokers := make(map[int]*models.Broker, 2)
		for _, id := range ids {
			var broker models.Broker
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, id).First(&broker).Error; err != nil {
				return err
			}
			if err := prepareCashBaseline(tx, userID, &broker); err != nil {
				return err
			}
			brokers[id] = &broker
		}
		from, to := brokers[fromID], brokers[toID]
		if from.Cash < amount {
			return errs.BadRequest("Broker From Does Not Have Enough Cash")
		}
		from.Cash -= amount
		from.CashBase -= amount
		to.Cash += amount
		to.CashBase += amount
		for _, id := range ids {
			if err := tx.Save(brokers[id]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
