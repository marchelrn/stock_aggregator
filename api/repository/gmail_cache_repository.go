package repository

import (
	"context"
	"time"

	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GmailCacheRepository struct {
	db *gorm.DB
}

func ImplGmailCacheRepository(db *gorm.DB) *GmailCacheRepository {
	return &GmailCacheRepository{db: db}
}

// IsProcessed checks if a Gmail message has already been processed for a user.
func (r *GmailCacheRepository) IsProcessed(ctx context.Context, userID uint, messageID, messageType string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ProcessedGmailMessage{}).
		Where("user_id = ? AND message_id = ? AND message_type = ?", userID, messageID, messageType).
		Count(&count).Error
	return count > 0, err
}

// MarkProcessed records that a Gmail message has been successfully processed.
func (r *GmailCacheRepository) MarkProcessed(ctx context.Context, userID uint, messageID, messageType string, tradeCount int) error {
	record := models.ProcessedGmailMessage{
		UserID:      userID,
		MessageID:   messageID,
		MessageType: messageType,
		ProcessedAt: time.Now(),
		TradeCount:  tradeCount,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "message_id"}, {Name: "message_type"}},
			DoUpdates: clause.Assignments(map[string]any{
				"processed_at": record.ProcessedAt,
				"trade_count":  record.TradeCount,
			}),
		}).
		Create(&record).Error
}

// GetProcessedMessageIDs returns a set of message IDs that have been processed for a user.
// This is more efficient than checking IsProcessed for each message individually.
func (r *GmailCacheRepository) GetProcessedMessageIDs(ctx context.Context, userID uint, messageType string) (map[string]bool, error) {
	var records []models.ProcessedGmailMessage
	err := r.db.WithContext(ctx).
		Select("message_id").
		Where("user_id = ? AND message_type = ?", userID, messageType).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(records))
	for _, r := range records {
		result[r.MessageID] = true
	}
	return result, nil
}

// ClearOldCache removes processed message records older than the given duration.
// This prevents the table from growing indefinitely.
func (r *GmailCacheRepository) ClearOldCache(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return r.db.WithContext(ctx).
		Where("processed_at < ?", cutoff).
		Delete(&models.ProcessedGmailMessage{}).Error
}
