package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TradeReviewRepository struct{ db *gorm.DB }

func (r *TradeReviewRepository) List(ctx context.Context, uid uint, includeSaved bool) ([]models.TradeReview, error) {
	if err := r.db.WithContext(ctx).Model(&models.TradeReview{}).
		Where("user_id=? AND state='saved'", uid).
		Where("NOT ("+importedRowsExist("trade_reviews", "transactions")+")").
		Update("state", "pending").Error; err != nil {
		return nil, err
	}
	rows := []models.TradeReview{}
	q := r.db.WithContext(ctx).Omit("draft").Where("user_id = ?", uid)
	if !includeSaved {
		q = q.Where("state <> 'saved'")
	}
	err := q.Order("id DESC").Find(&rows).Error
	return rows, err
}
func (r *TradeReviewRepository) Remember(ctx context.Context, row *models.TradeReview) (*models.TradeReview, error) {
	db := r.db.WithContext(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return nil, err
	}
	var found models.TradeReview
	err := db.Omit("draft").Where("user_id=? AND broker_id=? AND digest=?", row.UserID, row.BrokerID, row.Digest).First(&found).Error
	return &found, err
}
func (r *TradeReviewRepository) Get(ctx context.Context, uid, id uint) (*models.TradeReview, error) {
	var row models.TradeReview
	err := r.db.WithContext(ctx).Where("user_id=? AND id=?", uid, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.NotFound("Dokumen tidak ditemukan.")
	}
	return &row, err
}
func (r *TradeReviewRepository) RecordEmailSentAt(ctx context.Context, uid, id uint, sentAt time.Time) error {
	return r.db.WithContext(ctx).Exec(`UPDATE transactions AS t SET email_sent_at = ?
		FROM trade_reviews AS r
		WHERE r.user_id = ? AND r.id = ? AND r.state = 'saved'
		  AND t.user_id = r.user_id AND t.broker_id = r.broker_id
		  AND r.imported_ids @> jsonb_build_array(t.id)
		  AND (t.email_sent_at IS NULL OR t.email_sent_at > ?)`, sentAt, uid, id, sentAt).Error
}

func (r *TradeReviewRepository) BeginExtraction(ctx context.Context, uid, id uint, source, lease string) error {
	q := r.db.WithContext(ctx).Exec(`UPDATE trade_reviews SET consented_at=now(), consent_source=?, extraction_lease=?
 WHERE user_id=? AND id=? AND state='pending' AND (extraction_lease IS NULL OR consented_at < now()-interval '2 minutes')`, source, lease, uid, id)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return errs.Conflict("Dokumen sedang diproses atau sudah diperbarui. Buka kembali hasil review.")
	}
	return nil
}
func (r *TradeReviewRepository) EndExtraction(ctx context.Context, uid, id uint, lease string) error {
	return r.db.WithContext(ctx).Exec("UPDATE trade_reviews SET extraction_lease=NULL WHERE user_id=? AND id=? AND extraction_lease=?", uid, id, lease).Error
}
func (r *TradeReviewRepository) Draft(ctx context.Context, uid, id uint, lease, encrypted string) error {
	result := r.db.WithContext(ctx).Model(&models.TradeReview{}).Where("user_id=? AND id=? AND state='pending' AND extraction_lease=?", uid, id, lease).Updates(map[string]any{"state": "review", "draft": encrypted, "extraction_lease": nil})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errs.Conflict("Dokumen sudah diperbarui. Buka kembali hasil review.")
	}
	return nil
}
func (r *TradeReviewRepository) Save(ctx context.Context, uid, id uint, expected string, trades []*models.Transaction) (int, error) {
	var count int
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row models.TradeReview
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id=? AND id=?", uid, id).First(&row).Error; err != nil {
			return err
		}
		if row.State == "saved" {
			return nil
		}
		if row.State != expected {
			return errs.Conflict("Baca dokumen sebelum menyimpan transaksi.")
		}
		for _, trade := range trades {
			trade.BrokerID = row.BrokerID
			trade.BrokerName = row.BrokerName
			trade.UserID = uid
		}
		var err error
		count, err = (&TransactionRepository{db: tx}).SyncTradeConfirmationBatch(uid, trades)
		if err != nil {
			return err
		}
		ids := make([]int, 0, len(trades))
		for _, trade := range trades {
			if trade.Id <= 0 {
				return errors.New("saved transaction has no database ID")
			}
			ids = append(ids, trade.Id)
		}
		encoded, err := json.Marshal(ids)
		if err != nil {
			return err
		}
		return tx.Model(&row).Updates(map[string]any{"state": "saved", "draft": "", "imported_ids": json.RawMessage(encoded)}).Error
	})
	return count, err
}
