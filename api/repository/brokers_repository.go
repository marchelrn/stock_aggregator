package repository

import (
	"github.com/marchelrn/stock_api/models"
	"gorm.io/gorm"
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

func (r *BrokersRepository) UpdateBrokerData(userID uint, id int, payload *models.Broker) (*models.Broker, error) {
	var broker models.Broker
	if err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&broker).Error; err != nil {
		return nil, err
	}
	if payload.Name != "" {
		broker.Name = payload.Name
	}
	broker.Cash = payload.Cash

	if err := r.db.Save(&broker).Error; err != nil {
		return nil, err
	}
	return &broker, nil
}
