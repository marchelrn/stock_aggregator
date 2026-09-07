package service

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"gorm.io/gorm"
)

type BrokersService struct {
	BrokersRepository contract.BrokerRepository
}

func ImplBrokersService(repo contract.BrokerRepository) contract.BrokersService {
	return &BrokersService{BrokersRepository: repo}
}

func (s *BrokersService) TransferCashBetweenBrokers(payload *dto.TransferCashRequest) (*dto.BasicResponse, error) {
	brokerFrom, err := s.BrokersRepository.GetBrokerById(payload.FromBrokerId)
	if err != nil {
		return nil, errs.NotFound("Failed to update Broker : Brokers Not Found")
	}
	brokerTo, err := s.BrokersRepository.GetBrokerById(payload.ToBrokerId)
	if err != nil {
		return nil, errs.NotFound("Failed to update Broker : Brokers Not Found")
	}

	if brokerFrom.Cash < payload.Amount {
		return nil, errs.BadRequest("Broker From Does Not Have Enough Cash")
	}

	brokerFrom.Cash -= payload.Amount
	brokerTo.Cash += payload.Amount

	_, err = s.BrokersRepository.UpdateBrokerData(payload.FromBrokerId, brokerFrom)
	if err != nil {
		return nil, err
	}

	_, err = s.BrokersRepository.UpdateBrokerData(payload.ToBrokerId, brokerTo)
	if err != nil {
		return nil, err
	}

	response := &dto.BasicResponse{
		StatusCode: http.StatusOK,
		Message:    "Success transfer cash between brokers",
	}
	return response, nil
}

func (s *BrokersService) GetBroker(id int) (*dto.BasicBrokerResponse, error) {
	broker, err := s.BrokersRepository.GetBrokerById(id)
	if err != nil {
		return nil, errs.NotFound("Failed to get Broker : Broker Not Found")
	}

	response := &dto.BasicBrokerResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received Broker",
		Data: dto.Broker{
			Id:   broker.ID,
			Name: broker.Name,
			Cash: broker.Cash,
		},
	}
	return response, nil
}

func (s *BrokersService) GetAllBrokers() (*dto.BasicBrokersResponse, error) {
	brokers, err := s.BrokersRepository.GetAllBrokers()
	if err != nil {
		return nil, errs.NotFound("Failed to get Brokers : Brokers Not Found")
	}

	var brokersData []dto.Broker
	for _, broker := range brokers {
		brokersData = append(brokersData, dto.Broker{
			Id:   broker.ID,
			Name: broker.Name,
			Cash: broker.Cash,
		})
	}

	response := &dto.BasicBrokersResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received all Brokers",
		Data:       brokersData,
	}
	return response, nil
}

func (s *BrokersService) GetBrokersDetails(brokerName []string) (*dto.BrokersResponse, error) {
	brokers, err := s.BrokersRepository.GetBrokerDetails(brokerName)
	if err != nil {
		return nil, err
	}

	var brokerList []dto.BrokerDetails

	for _, broker := range brokers {
		var stocksData []dto.StocksData
		for _, stock := range broker.Stocks {
			stocksData = append(stocksData, dto.StocksData{
				Id:         stock.Id,
				Ticker:     stock.Ticker,
				Lot:        stock.Lot,
				AvgPrice:   stock.AvgPrice,
				BrokerID:   stock.BrokerID,
				BrokerName: stock.BrokerName,
				CreatedAt:  stock.CreatedAt,
				UpdatedAt:  stock.UpdatedAt,
			})
		}

		brokerList = append(brokerList, dto.BrokerDetails{
			Id:          broker.ID,
			Name:        broker.Name,
			Cash:        broker.Cash,
			StocksData:  stocksData,
			StocksCount: len(stocksData),
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

func (s *BrokersService) GetAllBrokersDetails() (*dto.BrokersResponse, error) {
	allBrokers, err := s.BrokersRepository.GetAllBrokers()
	if err != nil {
		return nil, err
	}

	holdings, err := s.BrokersRepository.GetAllBrokerHoldings()
	if err != nil {
		return nil, err
	}

	holdingsMap := make(map[int][]dto.StocksData)
	for _, stock := range holdings {
		holdingsMap[stock.BrokerID] = append(holdingsMap[stock.BrokerID], dto.StocksData{
			Id:         stock.Id,
			Ticker:     stock.Ticker,
			Lot:        stock.Lot,
			AvgPrice:   stock.AvgPrice,
			BrokerID:   stock.BrokerID,
			BrokerName: stock.BrokerName,
			CreatedAt:  stock.CreatedAt,
			UpdatedAt:  stock.UpdatedAt,
		})
	}

	brokerList := make([]dto.BrokerDetails, 0, len(allBrokers))
	for _, broker := range allBrokers {
		stocks := holdingsMap[broker.ID]
		if stocks == nil {
			stocks = []dto.StocksData{}
		}
		brokerList = append(brokerList, dto.BrokerDetails{
			Id:          broker.ID,
			Name:        broker.Name,
			Cash:        broker.Cash,
			StocksData:  stocks,
			StocksCount: len(stocks),
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

func (s *BrokersService) AddBrokerData(payload *dto.BrokerRequest) (*dto.BrokersResponse, error) {

	if payload.Cash <= 0 {
		payload.Cash = 0
	}

	if payload.Name == "" {
		return nil, errs.NotFound("Broker Name Is Required")
	}

	_, err := s.BrokersRepository.GetBrokerByName(payload.Name)
	if err == nil {
		return nil, errs.BadRequest("Cannot add Broker Data with the same Broker Name")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	broker := &models.Broker{
		Name: payload.Name,
		Cash: payload.Cash,
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
					Id:          brokerModels.ID,
					Name:        brokerModels.Name,
					Cash:        brokerModels.Cash,
					StocksData:  []dto.StocksData{},
					StocksCount: 0,
				},
			},
			CreatedAt: brokerModels.CreatedAt,
			UpdatedAt: brokerModels.UpdatedAt,
		},
	}
	return response, nil
}

func (s *BrokersService) DeleteBrokerData(brokerName []string) (*dto.BasicResponse, error) {
	_, err := s.BrokersRepository.DeleteBrokerData(brokerName)
	if err != nil {
		return nil, err
	}

	response := &dto.BasicResponse{
		StatusCode: http.StatusOK,
		Message:    "Success deleted Broker Data",
	}
	return response, nil
}

func (s *BrokersService) UpdateBrokerData(id int, payload *dto.BrokerRequest) (*dto.BasicResponse, error) {
	if payload.Cash <= 0 {
		payload.Cash = 0
	}

	brokerData, err := s.BrokersRepository.GetBrokerById(id)
	if err != nil {
		return nil, errs.NotFound("Failed to update Broker : Broker Not Found")
	}

	newDataCash := brokerData.Cash + payload.Cash

	broker := &models.Broker{
		Name: payload.Name,
		Cash: newDataCash,
	}

	broker, err = s.BrokersRepository.UpdateBrokerData(id, broker)
	if err != nil {
		return nil, err
	}

	response := &dto.BasicResponse{
		StatusCode: http.StatusOK,
		Message:    fmt.Sprintf("Success Update Broker %s", broker.Name),
	}
	return response, nil
}
