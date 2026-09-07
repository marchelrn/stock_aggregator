package service

import (
	"errors"
	"net/http"
	"strings"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"gorm.io/gorm"
)

type StocksService struct {
	StocksRepository      contract.StocksRepository
	BrokerRepository      contract.BrokerRepository
	TransactionRepository contract.TransactionRepository
}

func ImplStocksService(stocksRepo contract.StocksRepository, brokerRepo contract.BrokerRepository, txRepo contract.TransactionRepository) contract.StocksService {
	return &StocksService{StocksRepository: stocksRepo, BrokerRepository: brokerRepo, TransactionRepository: txRepo}
}

func (s *StocksService) GetAllStocks(limiter int) (*dto.StocksResponse, error) {
	param := limiter

	stocks, err := s.StocksRepository.GetAllStocks(param)
	if err != nil {
		return nil, errs.InternalServerError("Failed to retrieve stock data")
	}

	stocksData := make([]dto.StocksData, 0, len(stocks))
	for _, stock := range stocks {
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

	response := &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received all stocks details",
		StocksData: stocksData,
	}
	return response, nil
}

func (s *StocksService) GetStocks(stockTicker []string) (*dto.StocksResponse, error) {
	stocks, err := s.StocksRepository.GetStocks(stockTicker)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.NotFound("Stock not found")
		}
		return nil, errs.InternalServerError("Failed to retrieve stock data")
	}

	if len(stocks) == 0 {
		return nil, errs.NotFound("Stock not found")
	}

	stocksData := make([]dto.StocksData, 0, len(stocks))
	for _, stock := range stocks {
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

	response := &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received stocks details",
		StocksData: stocksData,
	}
	return response, nil
}

func (s *StocksService) AddStockData(payload *dto.StocksRequest) (*dto.StocksResponse, error) {
	stock := &dto.StocksData{
		Ticker:     strings.ToUpper(strings.TrimSpace(payload.Ticker)),
		Lot:        payload.Lot,
		AvgPrice:   payload.AvgPrice,
		BrokerID:   payload.BrokerID,
		BrokerName: payload.BrokerName,
	}

	if stock.Ticker == "" {
		return nil, errs.BadRequest("ticker is required")
	}
	if stock.Lot <= 0 {
		return nil, errs.BadRequest("lot must be greater than 0")
	}
	if stock.AvgPrice <= 0 {
		return nil, errs.BadRequest("avg_price must be greater than 0")
	}

	if stock.BrokerID == 0 {
		return nil, errs.BadRequest("broker_id is required")
	}

	if _, err := s.StocksRepository.GetStockByTickerAndBroker(stock.Ticker, stock.BrokerID); err == nil {
		return nil, errs.BadRequest("stock already exists for this broker, use PUT /stock/:ticker or POST /transaction")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.InternalServerError("Failed to validate existing stock")
	}

	addedStock, err := s.StocksRepository.AddStockData(&models.StockHolding{
		Ticker:     stock.Ticker,
		Lot:        stock.Lot,
		AvgPrice:   stock.AvgPrice,
		BrokerID:   stock.BrokerID,
		BrokerName: stock.BrokerName,
	})
	if err != nil {
		return nil, errs.InternalServerError("Failed to add stock data")
	}

	response := &dto.StocksResponse{
		StatusCode: http.StatusCreated,
		Message:    "Stock data added successfully",
		StocksData: []dto.StocksData{{
			Ticker:     addedStock.Ticker,
			Lot:        addedStock.Lot,
			AvgPrice:   addedStock.AvgPrice,
			BrokerID:   addedStock.BrokerID,
			BrokerName: addedStock.BrokerName,
			CreatedAt:  addedStock.CreatedAt,
			UpdatedAt:  addedStock.UpdatedAt,
		}},
	}
	return response, nil
}

func (s *StocksService) UpdateStock(stockTicker []string, payload *dto.StocksRequest) (*dto.StocksResponse, error) {
	stocks, err := s.StocksRepository.GetStocksReturnModels(stockTicker)
	if err != nil {
		return nil, err
	}

	if stocks == nil {
		return nil, errs.NotFound("Stock not found")
	}

	if payload.Ticker == "" {
		stocks.Ticker = payload.Ticker
	}

	if payload.Lot == 0 {
		stocks.Lot = payload.Lot
	}

	if payload.AvgPrice == 0 {
		stocks.AvgPrice = payload.AvgPrice
	}

	if payload.BrokerID == 0 {
		stocks.BrokerID = payload.BrokerID
	}

	if payload.BrokerName == "" {
		stocks.BrokerName = payload.BrokerName
	}

	updatedStock, err := s.StocksRepository.UpdateStockData(stockTicker, &models.StockHolding{
		Ticker:     payload.Ticker,
		Lot:        payload.Lot,
		AvgPrice:   payload.AvgPrice,
		BrokerID:   payload.BrokerID,
		BrokerName: payload.BrokerName,
	})
	if err != nil {
		return nil, errs.InternalServerError("Failed to update stock data")
	}

	response := &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Stock data updated successfully",
		StocksData: []dto.StocksData{{
			Id:         updatedStock.Id,
			Ticker:     updatedStock.Ticker,
			Lot:        updatedStock.Lot,
			AvgPrice:   updatedStock.AvgPrice,
			BrokerID:   updatedStock.BrokerID,
			BrokerName: updatedStock.BrokerName,
			CreatedAt:  updatedStock.CreatedAt,
			UpdatedAt:  updatedStock.UpdatedAt,
		}},
	}
	return response, nil
}

func (s *StocksService) DeleteStockData(stockTicker []string) (*dto.StocksResponse, error) {
	for _, ticker := range stockTicker {
		normalized := strings.ToUpper(strings.TrimSpace(ticker))
		if normalized == "" {
			continue
		}
		count, err := s.TransactionRepository.CountTransactionsByTicker(normalized)
		if err != nil {
			return nil, errs.InternalServerError("Failed to validate transaction history")
		}
		if count > 0 {
			return nil, errs.BadRequest("cannot delete stock with transaction history; keep holding or offset via SELL transaction")
		}
	}

	err := s.StocksRepository.DeleteStockData(stockTicker)
	if err != nil {
		return nil, errs.InternalServerError("Failed to delete stock data")
	}

	response := &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Stock data deleted successfully",
	}
	return response, nil
}
