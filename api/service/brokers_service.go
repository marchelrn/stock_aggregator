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
	brokerFrom, err := s.BrokersRepository.GetBrokerById(userID, payload.FromBrokerId)
	if err != nil {
		return nil, errs.NotFound("Failed to update Broker : Brokers Not Found")
	}
	brokerTo, err := s.BrokersRepository.GetBrokerById(userID, payload.ToBrokerId)
	if err != nil {
		return nil, errs.NotFound("Failed to update Broker : Brokers Not Found")
	}

	if brokerFrom.Cash < payload.Amount {
		return nil, errs.BadRequest("Broker From Does Not Have Enough Cash")
	}

	brokerFrom.Cash -= payload.Amount
	brokerTo.Cash += payload.Amount

	_, err = s.BrokersRepository.UpdateBrokerData(userID, payload.FromBrokerId, brokerFrom)
	if err != nil {
		return nil, err
	}

	_, err = s.BrokersRepository.UpdateBrokerData(userID, payload.ToBrokerId, brokerTo)
	if err != nil {
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
			Id:   broker.ID,
			Name: broker.Name,
			Cash: broker.Cash,
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

func (s *BrokersService) GetBrokersDetails(userID uint, brokerName []string) (*dto.BrokersResponse, error) {
	brokers, err := s.BrokersRepository.GetBrokerDetails(userID, brokerName)
	if err != nil {
		return nil, err
	}

	var brokerList []dto.BrokerDetails

	for _, broker := range brokers {
		stocksData := s.convertStocksToDTO(broker.Stocks)

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

	brokerData, err := s.BrokersRepository.GetBrokerById(userID, id)
	if err != nil {
		return nil, errs.NotFound("Failed to update Broker : Broker Not Found")
	}

	newDataCash := brokerData.Cash + payload.Cash

	broker := &models.Broker{
		UserID: userID,
		Name:   payload.Name,
		Cash:   newDataCash,
	}

	broker, err = s.BrokersRepository.UpdateBrokerData(userID, id, broker)
	if err != nil {
		return nil, err
	}

	response := &dto.BasicResponse{
		StatusCode: http.StatusOK,
		Message:    fmt.Sprintf("Success Update Broker %s", broker.Name),
	}
	return response, nil
}
