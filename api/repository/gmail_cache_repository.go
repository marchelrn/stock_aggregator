package repository

import (
	"context"
	"encoding/json"
	"strings"
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
	ids, err := r.GetProcessedMessageIDs(ctx, userID, messageType)
	return ids[messageID], err
}

// MarkProcessed records that a Gmail message has been successfully processed.
func (r *GmailCacheRepository) MarkProcessed(ctx context.Context, userID uint, messageID, messageType string, tradeCount int) error {
	return r.MarkVerified(ctx, userID, messageID, messageType, tradeCount, nil)
}

func (r *GmailCacheRepository) MarkVerified(ctx context.Context, userID uint, messageID, messageType string, tradeCount int, ids []int) error {
	encoded, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	record := models.ProcessedGmailMessage{
		UserID:      userID,
		MessageID:   messageID,
		MessageType: messageType,
		ProcessedAt: time.Now(),
		TradeCount:  tradeCount,
		ImportedIDs: encoded,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "message_id"}, {Name: "message_type"}},
			DoUpdates: clause.Assignments(map[string]any{
				"processed_at": record.ProcessedAt,
				"trade_count":  record.TradeCount,
				"imported_ids": record.ImportedIDs,
			}),
		}).
		Create(&record).Error
}

// GetProcessedMessageIDs returns a set of message IDs that have been processed for a user.
// This is more efficient than checking IsProcessed for each message individually.
func (r *GmailCacheRepository) GetProcessedMessageIDs(ctx context.Context, userID uint, messageType string) (map[string]bool, error) {
	table := "transactions"
	if strings.HasPrefix(messageType, "cash_mutation:") {
		table = "cash_mutations"
	}
	var records []models.ProcessedGmailMessage
	err := r.db.WithContext(ctx).
		Select("message_id").
		Where("user_id = ? AND message_type = ?", userID, messageType).
		Where(importedRowsExist("processed_gmail_messages", table)).
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

func importedRowsExist(source, target string) string {
	return `CASE WHEN jsonb_typeof(` + source + `.imported_ids) = 'array' THEN
		jsonb_array_length(` + source + `.imported_ids) > 0 AND NOT EXISTS (
			SELECT 1 FROM jsonb_array_elements_text(` + source + `.imported_ids) AS imported(id)
			WHERE NOT EXISTS (SELECT 1 FROM ` + target + ` AS actual
				WHERE actual.id = imported.id::bigint AND actual.user_id = ` + source + `.user_id)
		) ELSE false END`
}

// ClearOldCache removes processed message records older than the given duration.
// This prevents the table from growing indefinitely.
func (r *GmailCacheRepository) ClearOldCache(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return r.db.WithContext(ctx).
		Where("processed_at < ?", cutoff).
		Delete(&models.ProcessedGmailMessage{}).Error
}
