package repository

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Cash starts from a manually supplied balance; history before registration
// or the latest current-balance reset must never be replayed into it.

// TradeCashDelta menghitung pengaruh kas dari satu isi transaksi:
// BUY = -(gross + tax), SELL = gross - tax.
func TradeCashDelta(t *models.Transaction) float64 {
	gross := float64(t.Lot) * 100 * t.Price
	tax := t.Tax
	if tax == 0 {
		if t.Type == models.TypeBuy {
			tax = gross * 0.0015
		} else {
			tax = gross * 0.0025
		}
	}
	if t.Type == models.TypeBuy {
		return -(gross + tax)
	}
	return gross - tax
}

// MutationCashDelta menghitung pengaruh kas dari sebuah mutasi.
func MutationCashDelta(m *models.CashMutation) float64 {
	switch m.Type {
	case models.CashMutationDeposit, models.CashMutationDividend:
		return m.Amount
	case models.CashMutationWithdraw:
		return -m.Amount
	}
	return 0
}

// Email receipt is import metadata, not the time money moved.
func CashEventTime(m *models.CashMutation) time.Time {
	return m.Date
}

// CountsAfterCutoff menentukan apakah sebuah event ikut mengubah cash.
// A missing cutoff fails closed; callers must resolve the registration floor.
func CountsAfterCutoff(eventTime time.Time, asOf *time.Time) bool {
	if asOf == nil || asOf.IsZero() || eventTime.IsZero() {
		return false
	}
	return eventTime.After(*asOf)
}

// cashEventsAfter mengumpulkan seluruh event kas milik sebuah broker yang
// terjadi setelah cutoff (transaksi + mutasi), diurutkan kronologis,
// beserta total dampak kasnya.
func cashEventsAfter(db *gorm.DB, userID uint, brokerID int, asOf *time.Time) ([]dto.CashEvent, float64, error) {
	if asOf == nil || asOf.IsZero() {
		return []dto.CashEvent{}, 0, nil
	}
	var trades []models.Transaction
	tradeQ := db.Where("user_id = ? AND broker_id = ? AND date > ?", userID, brokerID, *asOf)
	if err := tradeQ.Find(&trades).Error; err != nil {
		return nil, 0, err
	}

	var mutations []models.CashMutation
	mutQ := db.Where("user_id = ? AND broker_id = ? AND date > ?", userID, brokerID, *asOf)
	if err := mutQ.Find(&mutations).Error; err != nil {
		return nil, 0, err
	}

	events := make([]dto.CashEvent, 0, len(trades)+len(mutations))
	var total float64

	for i := range trades {
		t := &trades[i]
		delta := TradeCashDelta(t)
		total += delta
		events = append(events, dto.CashEvent{
			Source:      "TRADE",
			Type:        string(t.Type),
			Delta:       delta,
			Date:        t.Date,
			Description: fmt.Sprintf("%s %s %d lot @ %.2f", t.Type, t.Ticker, t.Lot, t.Price),
		})
	}

	for i := range mutations {
		m := &mutations[i]
		delta := MutationCashDelta(m)
		total += delta
		desc := m.Description
		if desc == "" {
			desc = string(m.Type)
		}
		events = append(events, dto.CashEvent{
			Source:      "MUTATION",
			Type:        string(m.Type),
			Delta:       delta,
			Date:        CashEventTime(m),
			Description: desc,
		})
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Date.Before(events[j].Date)
	})

	return events, total, nil
}

func registrationTime(db *gorm.DB, userID uint) (time.Time, error) {
	var user models.Users
	if err := db.Select("id", "created_at").First(&user, userID).Error; err != nil {
		return time.Time{}, err
	}
	if user.CreatedAt.IsZero() {
		return time.Time{}, fmt.Errorf("user registration time is missing; cash changes are unsafe")
	}
	return user.CreatedAt, nil
}

// Resolve legacy metadata without guessing a corrected balance. The derived
// base preserves stored cash, including any contamination that needs user review.
// Call before inserting events, while holding the broker lock on write paths.
func prepareCashBaseline(db *gorm.DB, userID uint, broker *models.Broker) error {
	registered, err := registrationTime(db, userID)
	if err != nil {
		return err
	}
	if broker.CashBaseAsOf != nil && !broker.CashBaseAsOf.Before(registered) {
		return nil
	}
	_, total, err := cashEventsAfter(db, userID, broker.ID, &registered)
	if err != nil {
		return err
	}
	broker.CashBase = broker.Cash - total
	broker.CashBaseAsOf = &registered
	return nil
}

// RecomputeBrokerCash updates the in-memory broker; the caller persists it.
func RecomputeBrokerCash(db *gorm.DB, userID uint, broker *models.Broker) error {
	if err := prepareCashBaseline(db, userID, broker); err != nil {
		return err
	}
	_, total, err := cashEventsAfter(db, userID, broker.ID, broker.CashBaseAsOf)
	if err != nil {
		return err
	}
	broker.Cash = broker.CashBase + total
	return nil
}

// SetCashBase resets the current balance under lock. Legacy asOf input is
// deliberately ignored: backdated or future resets are not supported.
func (r *BrokersRepository) SetCashBase(userID uint, id int, cashBase float64, asOf time.Time) (*models.Broker, error) {
	if cashBase < 0 || math.IsNaN(cashBase) || math.IsInf(cashBase, 0) {
		return nil, errs.BadRequest("Cash balance must be finite and nonnegative")
	}
	var broker models.Broker
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND id = ?", userID, id).
			First(&broker).Error; err != nil {
			return err
		}
		registered, err := registrationTime(tx, userID)
		if err != nil {
			return err
		}
		asOf = time.Now().UTC()
		if registered.After(asOf) {
			asOf = registered
		}
		broker.Cash = cashBase
		broker.CashBase = cashBase
		broker.CashBaseAsOf = &asOf
		return tx.Save(&broker).Error
	})
	if err != nil {
		return nil, err
	}
	return &broker, nil
}

// GetCashBreakdown menghitung rincian saldo kas sebuah broker secara live:
// saldo dasar + daftar event setelah cutoff = cash.
func (r *BrokersRepository) GetCashBreakdown(userID uint, id int) (*dto.CashBreakdown, error) {
	var broker models.Broker
	if err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&broker).Error; err != nil {
		return nil, err
	}

	if err := prepareCashBaseline(r.db, userID, &broker); err != nil {
		return nil, err
	}
	events, total, err := cashEventsAfter(r.db, userID, broker.ID, broker.CashBaseAsOf)
	if err != nil {
		return nil, err
	}

	cash := broker.CashBase + total

	return &dto.CashBreakdown{
		BrokerId:     broker.ID,
		BrokerName:   broker.Name,
		Cash:         cash,
		CashBase:     broker.CashBase,
		CashBaseAsOf: broker.CashBaseAsOf,
		EventsTotal:  total,
		Events:       events,
	}, nil
}
