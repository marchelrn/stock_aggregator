package repository

import (
	"math"

	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BrokersRepository struct {
	db *gorm.DB
}

func ImplBrokersRepository(db *gorm.DB) *BrokersRepository {
	return &BrokersRepository{db: db}
}

func (r *BrokersRepository) GetBrokerDetails(userID uint, brokerName []string) ([]models.BrokerDetails, error) {
	var brokerDetails []models.BrokerDetails
	if err := r.db.Preload("Stocks", "user_id = ? AND lot > 0", userID).Where("user_id = ? AND name IN ?", userID, brokerName).Find(&brokerDetails).Error; err != nil {
		return nil, err
	}
	return brokerDetails, nil
}

func (r *BrokersRepository) GetAllBrokerHoldings(userID uint) ([]models.StockHolding, error) {
	var holdings []models.StockHolding
	if err := r.db.Preload("Broker").Where("user_id = ? AND lot > 0", userID).Order("broker_id ASC, id ASC").Find(&holdings).Error; err != nil {
		return nil, err
	}
	return holdings, nil
}

func (r *BrokersRepository) AddBrokerData(payload *models.Broker) (*models.Broker, error) {
	if payload.Cash < 0 || math.IsNaN(payload.Cash) || math.IsInf(payload.Cash, 0) {
		return nil, errs.BadRequest("Initial cash must be finite and nonnegative")
	}
	registered, err := registrationTime(r.db, payload.UserID)
	if err != nil {
		return nil, err
	}
	payload.CashBase = payload.Cash
	payload.CashBaseAsOf = &registered
	if err := r.db.Create(payload).Error; err != nil {
		return nil, err
	}
	return payload, nil
}

func (r *BrokersRepository) DeleteBrokerData(userID uint, brokerName []string) (*models.Broker, error) {
	var broker models.Broker
	if err := r.db.Where("user_id = ? AND name IN ?", userID, brokerName).First(&broker).Error; err != nil {
		return nil, err
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND broker_id = ?", userID, broker.ID).Delete(&models.StockHolding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND id = ?", userID, broker.ID).Delete(&broker).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &broker, nil
}

func (r *BrokersRepository) GetAllBrokers(userID uint) ([]models.Broker, error) {
	var brokers []models.Broker
	if err := r.db.Where("user_id = ?", userID).Find(&brokers).Error; err != nil {
		return nil, err
	}
	return brokers, nil
}

func (r *BrokersRepository) GetBrokerByName(userID uint, name string) (*models.Broker, error) {
	var broker models.Broker
	if err := r.db.Where("user_id = ? AND name = ?", userID, name).First(&broker).Error; err != nil {
		return nil, err
	}
	return &broker, nil
}

func (r *BrokersRepository) GetBrokerById(userID uint, id int) (*models.Broker, error) {
	var broker models.Broker
	if err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&broker).Error; err != nil {
		return nil, err
	}
	return &broker, nil
}

// UpdateBrokerData atomically adds payload.Cash to the current balance.
func (r *BrokersRepository) UpdateBrokerData(userID uint, id int, payload *models.Broker) (*models.Broker, error) {
	if payload.Cash < 0 || math.IsNaN(payload.Cash) || math.IsInf(payload.Cash, 0) {
		return nil, errs.BadRequest("Cash addition must be finite and nonnegative")
	}
	var broker models.Broker
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, id).First(&broker).Error; err != nil {
			return err
		}
		if payload.PDFPasswordEncrypted == "" || payload.Cash > 0 {
			if err := prepareCashBaseline(tx, userID, &broker); err != nil {
				return err
			}
		}
		if payload.PDFPasswordEncrypted != "" {
			broker.PDFPasswordEncrypted = payload.PDFPasswordEncrypted
		}
		if payload.Name != "" {
			broker.Name = payload.Name
		}
		broker.Cash += payload.Cash
		broker.CashBase += payload.Cash
		return tx.Save(&broker).Error
	})
	if err != nil {
		return nil, err
	}
	return &broker, nil
}
