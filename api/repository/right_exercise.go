package repository

import (
	"context"
	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type rightExercises struct{ db *gorm.DB }

func (r rightExercises) Record(ctx context.Context, rows []models.RightExercise) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}
