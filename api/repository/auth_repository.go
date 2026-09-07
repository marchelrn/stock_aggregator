package repository

import (
	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
)

type AuthRepository struct {
	db *gorm.DB
}

func ImplAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) FindByEmail(email string) (*models.Users, error) {
	var user models.Users
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindByID(id uint) (*models.Users, error) {
	var user models.Users
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindByProviderID(providerID string) (*models.Users, error) {
	var user models.Users
	if err := r.db.Where("provider_id = ?", providerID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) CreateUser(user *models.Users) (*models.Users, error) {
	if err := r.db.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *AuthRepository) UpdateUser(user *models.Users) (*models.Users, error) {
	if err := r.db.Save(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}
