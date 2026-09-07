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

func (r *BrokersRepository) GetBrokerDetails(brokerName []string) ([]models.BrokerDetails, error) {
	var brokerDetails []models.BrokerDetails
	if err := r.db.Preload("Stocks").Where("name IN ?", brokerName).Find(&brokerDetails).Error; err != nil {
		return nil, err
	}
	return brokerDetails, nil
}

func (r *BrokersRepository) GetAllBrokerHoldings() ([]models.StockHolding, error) {
	var holdings []models.StockHolding
	if err := r.db.Preload("Broker").Order("broker_id ASC, id ASC").Find(&holdings).Error; err != nil {
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

func (r *BrokersRepository) DeleteBrokerData(brokerName []string) (*models.Broker, error) {
	var broker models.Broker
	if err := r.db.Where("name IN ?", brokerName).First(&broker).Error; err != nil {
		return nil, err
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("broker_id = ?", broker.ID).Delete(&models.StockHolding{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&broker).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &broker, nil
}

func (r *BrokersRepository) GetAllBrokers() ([]models.Broker, error) {
	var brokers []models.Broker
	if err := r.db.Find(&brokers).Error; err != nil {
		return nil, err
	}
	return brokers, nil
}

func (r *BrokersRepository) GetBrokerByName(name string) (*models.Broker, error) {
	var broker models.Broker
	if err := r.db.Where("name = ?", name).First(&broker).Error; err != nil {
		return nil, err
	}
	return &broker, nil
}

func (r *BrokersRepository) GetBrokerById(id int) (*models.Broker, error) {
	var broker models.Broker
	if err := r.db.Where("id = ?", id).First(&broker).Error; err != nil {
		return nil, err
	}
	return &broker, nil
}

func (r *BrokersRepository) UpdateBrokerData(id int, payload *models.Broker) (*models.Broker, error) {
	var broker models.Broker
	if err := r.db.Where("id = ?", id).First(&broker).Error; err != nil {
		return nil, err
	}
	if payload.Name != "" {
		broker.Name = payload.Name
	}
	if payload.Cash == 0 {
		broker.Cash = payload.Cash
	}
	broker.Cash = payload.Cash

	if err := r.db.Save(&broker).Error; err != nil {
		return nil, err
	}
	return &broker, nil
}
