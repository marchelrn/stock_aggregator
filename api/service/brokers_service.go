package service

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"github.com/marchelrn/stock_api/utils"
	"gorm.io/gorm"
)

type BrokersService struct {
	BrokersRepository contract.BrokerRepository
}

func ImplBrokersService(repo contract.BrokerRepository) contract.BrokersService {
	return &BrokersService{BrokersRepository: repo}
}

// convertStockToDTO converts a single stock model to DTO format
func (s *BrokersService) convertStockToDTO(stock models.StockHolding) dto.StocksData {
	return dto.StocksData{
		Id:         stock.Id,
		Ticker:     stock.Ticker,
		Sector:     stock.Sector,
		Lot:        stock.Lot,
		AvgPrice:   stock.AvgPrice,
		BrokerID:   stock.BrokerID,
		BrokerName: stock.BrokerName,
		CreatedAt:  stock.CreatedAt,
		UpdatedAt:  stock.UpdatedAt,
	}
}

// convertStocksToDTO converts multiple stocks to DTO format
func (s *BrokersService) convertStocksToDTO(stocks []models.StockHolding) []dto.StocksData {
	stocksData := make([]dto.StocksData, 0, len(stocks))
	for _, stock := range stocks {
		stocksData = append(stocksData, s.convertStockToDTO(stock))
	}
	return stocksData
}

func (s *BrokersService) TransferCashBetweenBrokers(userID uint, payload *dto.TransferCashRequest) (*dto.BasicResponse, error) {
	if err := s.BrokersRepository.TransferCash(userID, payload.FromBrokerId, payload.ToBrokerId, payload.Amount); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.NotFound("Failed to transfer cash : Broker Not Found")
		}
		return nil, err
	}

	response := &dto.BasicResponse{
		StatusCode: http.StatusOK,
		Message:    "Success transfer cash between brokers",
	}
	return response, nil
}

func (s *BrokersService) GetBroker(userID uint, id int) (*dto.BasicBrokerResponse, error) {
	broker, err := s.BrokersRepository.GetBrokerById(userID, id)
	if err != nil {
		return nil, errs.NotFound("Failed to get Broker : Broker Not Found")
	}

	response := &dto.BasicBrokerResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received Broker",
		Data: dto.Broker{
			Id:           broker.ID,
			Name:         broker.Name,
			Cash:         broker.Cash,
			CashBase:     broker.CashBase,
			CashBaseAsOf: broker.CashBaseAsOf,
		},
	}
	return response, nil
}

func (s *BrokersService) GetAllBrokers(userID uint) (*dto.BasicBrokersResponse, error) {
	brokers, err := s.BrokersRepository.GetAllBrokers(userID)
	if err != nil {
		return nil, errs.NotFound("Failed to get Brokers : Brokers Not Found")
	}

	var brokersData []dto.Broker
	for _, broker := range brokers {
		brokersData = append(brokersData, dto.Broker{
			Id:           broker.ID,
			Name:         broker.Name,
			Cash:         broker.Cash,
			CashBase:     broker.CashBase,
			CashBaseAsOf: broker.CashBaseAsOf,
		})
	}

	response := &dto.BasicBrokersResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received all Brokers",
		Data:       brokersData,
	}
	return response, nil
}

func (s *BrokersService) GetBrokersDetails(userID uint, brokerName []string) (*dto.BrokersResponse, error) {
	brokers, err := s.BrokersRepository.GetBrokerDetails(userID, brokerName)
	if err != nil {
		return nil, err
	}

	var brokerList []dto.BrokerDetails

	for _, broker := range brokers {
		stocksData := s.convertStocksToDTO(broker.Stocks)

		brokerList = append(brokerList, dto.BrokerDetails{
			Id:           broker.ID,
			Name:         broker.Name,
			Cash:         broker.Cash,
			CashBase:     broker.CashBase,
			CashBaseAsOf: broker.CashBaseAsOf,
			StocksData:   stocksData,
			StocksCount:  len(stocksData),
		})
	}

	response := &dto.BrokersResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received Brokers Details",
		BrokersData: dto.BrokersData{
			Brokers: brokerList,
		},
	}
	return response, nil
}

func (s *BrokersService) GetAllBrokersDetails(userID uint) (*dto.BrokersResponse, error) {
	allBrokers, err := s.BrokersRepository.GetAllBrokers(userID)
	if err != nil {
		return nil, err
	}

	holdings, err := s.BrokersRepository.GetAllBrokerHoldings(userID)
	if err != nil {
		return nil, err
	}

	holdingsMap := make(map[int][]dto.StocksData)
	for _, stock := range holdings {
		holdingsMap[stock.BrokerID] = append(holdingsMap[stock.BrokerID], s.convertStockToDTO(stock))
	}

	brokerList := make([]dto.BrokerDetails, 0, len(allBrokers))
	for _, broker := range allBrokers {
		stocks := holdingsMap[broker.ID]
		if stocks == nil {
			stocks = []dto.StocksData{}
		}
		brokerList = append(brokerList, dto.BrokerDetails{
			Id:           broker.ID,
			Name:         broker.Name,
			Cash:         broker.Cash,
			CashBase:     broker.CashBase,
			CashBaseAsOf: broker.CashBaseAsOf,
			StocksData:   stocks,
			StocksCount:  len(stocks),
		})
	}

	response := &dto.BrokersResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received all Brokers from stock holding",
		BrokersData: dto.BrokersData{
			Brokers: brokerList,
		},
	}
	return response, nil
}

func (s *BrokersService) AddBrokerData(userID uint, payload *dto.BrokerRequest) (*dto.BrokersResponse, error) {
	if payload.Cash <= 0 {
		payload.Cash = 0
	}

	if payload.Name == "" {
		return nil, errs.NotFound("Broker Name Is Required")
	}

	_, err := s.BrokersRepository.GetBrokerByName(userID, payload.Name)
	if err == nil {
		return nil, errs.BadRequest("Cannot add Broker Data with the same Broker Name")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	broker := &models.Broker{
		UserID: userID,
		Name:   payload.Name,
		Cash:   payload.Cash,
	}
	if payload.Pin != "" {
		encrypted, err := utils.Encrypt(payload.Pin, config.GetConfig().EncryptionKey)
		if err != nil {
			return nil, errs.InternalServerError("Gagal menyimpan password PDF")
		}
		broker.PDFPasswordEncrypted = encrypted
	}

	brokerModels, err := s.BrokersRepository.AddBrokerData(broker)
	if err != nil {
		return nil, err
	}

	response := &dto.BrokersResponse{
		StatusCode: http.StatusOK,
		Message:    "Success added Broker Data",
		BrokersData: dto.BrokersData{
			Brokers: []dto.BrokerDetails{
				{
					Id:           brokerModels.ID,
					Name:         brokerModels.Name,
					Cash:         brokerModels.Cash,
					CashBase:     brokerModels.CashBase,
					CashBaseAsOf: brokerModels.CashBaseAsOf,
					StocksData:   []dto.StocksData{},
					StocksCount:  0,
				},
			},
			CreatedAt: brokerModels.CreatedAt,
			UpdatedAt: brokerModels.UpdatedAt,
		},
	}
	return response, nil
}

func (s *BrokersService) DeleteBrokerData(userID uint, brokerName []string) (*dto.BasicResponse, error) {
	_, err := s.BrokersRepository.DeleteBrokerData(userID, brokerName)
	if err != nil {
		return nil, err
	}

	response := &dto.BasicResponse{
		StatusCode: http.StatusOK,
		Message:    "Success deleted Broker Data",
	}
	return response, nil
}

func (s *BrokersService) UpdateBrokerData(userID uint, id int, payload *dto.BrokerRequest) (*dto.BasicResponse, error) {
	if payload.Cash <= 0 {
		payload.Cash = 0
	}

	update := &models.Broker{
		UserID: userID,
		Name:   payload.Name,
		Cash:   payload.Cash,
	}
	if payload.Pin != "" {
		encrypted, err := utils.Encrypt(payload.Pin, config.GetConfig().EncryptionKey)
		if err != nil {
			return nil, errs.InternalServerError("Gagal menyimpan password PDF")
		}
		update.PDFPasswordEncrypted = encrypted
	}
	broker, err := s.BrokersRepository.UpdateBrokerData(userID, id, update)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.NotFound("Failed to update Broker : Broker Not Found")
		}
		return nil, err
	}

	response := &dto.BasicResponse{
		StatusCode: http.StatusOK,
		Message:    fmt.Sprintf("Success Update Broker %s", broker.Name),
	}
	return response, nil
}

// SetCashBase sets today's balance, never a historical replay starting point.
func (s *BrokersService) SetCashBase(userID uint, id int, payload *dto.SetCashBaseRequest) (*dto.CashBreakdownResponse, error) {
	if payload.CashBase < 0 || math.IsNaN(payload.CashBase) || math.IsInf(payload.CashBase, 0) {
		return nil, errs.BadRequest("Saldo dasar tidak boleh negatif")
	}

	asOf := time.Now().UTC()

	if _, err := s.BrokersRepository.SetCashBase(userID, id, payload.CashBase, asOf); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.NotFound("Failed to set cash base : Broker Not Found")
		}
		return nil, err
	}

	breakdown, err := s.BrokersRepository.GetCashBreakdown(userID, id)
	if err != nil {
		return nil, err
	}

	return &dto.CashBreakdownResponse{
		StatusCode: http.StatusOK,
		Message:    "Saldo kas saat ini disimpan",
		Data:       *breakdown,
	}, nil
}

// GetCashBreakdown mengembalikan rincian saldo kas: saldo dasar, event
// setelah cutoff, dan cash hasil hitungannya.
func (s *BrokersService) GetCashBreakdown(userID uint, id int) (*dto.CashBreakdownResponse, error) {
	breakdown, err := s.BrokersRepository.GetCashBreakdown(userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.NotFound("Failed to get cash breakdown : Broker Not Found")
		}
		return nil, err
	}

	return &dto.CashBreakdownResponse{
		StatusCode: http.StatusOK,
		Message:    "Rincian saldo kas",
		Data:       *breakdown,
	}, nil
}
