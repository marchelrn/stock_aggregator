package repository

import (
	"fmt"
	"sort"
	"time"

	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Model saldo kas:
//
//	cash = cash_base + SUM(delta event setelah cash_base_as_of)
//
// cash_base di-set manual oleh user (lihat SetCashBase). Selama
// cash_base_as_of masih NULL, broker berada di mode lama: setiap event
// langsung mengubah cash (perilaku replay historis seperti sebelumnya).

// TradeCashDelta menghitung pengaruh kas dari satu isi transaksi:
// BUY = -(gross + tax), SELL = gross - tax.
// Rumusnya identik dengan yang dipakai CalculateHoldingAfterBatch,
// sehingga mode lama (tanpa cutoff) menghasilkan angka yang sama.
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

// CashEventTime adalah waktu efektif sebuah mutasi kas: waktu email
// diterima dari broker bila tersedia, jika tidak pakai tanggal mutasi.
func CashEventTime(m *models.CashMutation) time.Time {
	if m.EmailReceivedAt != nil {
		return *m.EmailReceivedAt
	}
	return m.Date
}

// CountsAfterCutoff menentukan apakah sebuah event ikut mengubah cash.
// asOf NULL = mode lama, semua event diterapkan.
func CountsAfterCutoff(eventTime time.Time, asOf *time.Time) bool {
	if asOf == nil {
		return true
	}
	return eventTime.After(*asOf)
}

// cashEventsAfter mengumpulkan seluruh event kas milik sebuah broker yang
// terjadi setelah cutoff (transaksi + mutasi), diurutkan kronologis,
// beserta total dampak kasnya.
func cashEventsAfter(db *gorm.DB, userID uint, brokerID int, asOf *time.Time) ([]dto.CashEvent, float64, error) {
	var trades []models.Transaction
	tradeQ := db.Where("user_id = ? AND broker_id = ?", userID, brokerID)
	if asOf != nil {
		tradeQ = tradeQ.Where("date > ?", *asOf)
	}
	if err := tradeQ.Find(&trades).Error; err != nil {
		return nil, 0, err
	}

	var mutations []models.CashMutation
	mutQ := db.Where("user_id = ? AND broker_id = ?", userID, brokerID)
	if asOf != nil {
		mutQ = mutQ.Where("COALESCE(email_received_at, date) > ?", *asOf)
	}
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

// RecomputeBrokerCash menghitung ulang brokers.cash dari saldo dasar dan
// event setelah cutoff. Tidak melakukan apa-apa bila cutoff belum diatur.
func RecomputeBrokerCash(db *gorm.DB, userID uint, broker *models.Broker) error {
	if broker.CashBaseAsOf == nil {
		return nil
	}
	_, total, err := cashEventsAfter(db, userID, broker.ID, broker.CashBaseAsOf)
	if err != nil {
		return err
	}
	broker.Cash = broker.CashBase + total
	return nil
}

// SetCashBase mengatur saldo dasar sebuah broker pada waktu tertentu,
// lalu menghitung ulang cash = cash_base + event setelah cutoff.
func (r *BrokersRepository) SetCashBase(userID uint, id int, cashBase float64, asOf time.Time) (*models.Broker, error) {
	var broker models.Broker
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND id = ?", userID, id).
			First(&broker).Error; err != nil {
			return err
		}
		broker.CashBase = cashBase
		broker.CashBaseAsOf = &asOf
		if err := RecomputeBrokerCash(tx, userID, &broker); err != nil {
			return err
		}
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

	events, total, err := cashEventsAfter(r.db, userID, broker.ID, broker.CashBaseAsOf)
	if err != nil {
		return nil, err
	}

	cash := broker.Cash
	if broker.CashBaseAsOf != nil {
		cash = broker.CashBase + total
	}

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
