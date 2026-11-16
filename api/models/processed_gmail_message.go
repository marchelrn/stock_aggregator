package models

import "time"

// ProcessedGmailMessage tracks which Gmail messages have been successfully processed
// to avoid re-parsing during full sync.
type ProcessedGmailMessage struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UserID      uint      `json:"user_id" gorm:"not null;index:idx_gmail_cache_user_msgtype,priority:1"`
	MessageID   string    `json:"message_id" gorm:"not null;index:idx_gmail_cache_user_msgtype,priority:2"`
	MessageType string    `json:"message_type" gorm:"not null;index:idx_gmail_cache_user_msgtype,priority:3"` // "trade_confirmation" or "cash_mutation"
	ProcessedAt time.Time `json:"processed_at" gorm:"not null"`
	TradeCount  int       `json:"trade_count" gorm:"not null;default:0"`
}

func (ProcessedGmailMessage) TableName() string {
	return "processed_gmail_messages"
}
