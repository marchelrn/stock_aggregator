package repository

import (
	"context"
	"fmt"

	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
)

type PDFTemplateRepository struct {
	db *gorm.DB
}

func ImplPDFTemplateRepository(db *gorm.DB) *PDFTemplateRepository {
	return &PDFTemplateRepository{db: db}
}

func (r *PDFTemplateRepository) ListActive(ctx context.Context) ([]models.PDFParsingTemplate, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("PDF template repository: database is nil")
	}
	var templates []models.PDFParsingTemplate
	err := r.db.WithContext(ctx).Where("active = ?", true).
		Order("key ASC, version ASC").Find(&templates).Error
	return templates, err
}
