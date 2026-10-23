package models

import (
	"encoding/json"
	"time"
)

// PDFParsingTemplate stores a versioned, independently editable PDF layout.
type PDFParsingTemplate struct {
	ID              uint            `json:"id" gorm:"primaryKey"`
	Key             string          `json:"key" gorm:"not null;uniqueIndex:idx_pdf_template_key_version"`
	BrokerName      string          `json:"broker_name" gorm:"not null"`
	Version         int             `json:"version" gorm:"not null;uniqueIndex:idx_pdf_template_key_version"`
	Active          bool            `json:"active" gorm:"not null"`
	Config          json.RawMessage `json:"config" gorm:"type:jsonb;not null"`
	EmailSender     string          `json:"email_sender" gorm:"not null;default:''"`
	SubjectContains string          `json:"subject_contains" gorm:"not null;default:''"`
	FilenameSuffix  string          `json:"filename_suffix" gorm:"not null;default:''"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (PDFParsingTemplate) TableName() string { return "pdf_parsing_templates" }
