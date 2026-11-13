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
	YahooService          contract.YahooService
}

func ImplStocksService(stocksRepo contract.StocksRepository, brokerRepo contract.BrokerRepository, txRepo contract.TransactionRepository, yahooSvc contract.YahooService) contract.StocksService {
	return &StocksService{StocksRepository: stocksRepo, BrokerRepository: brokerRepo, TransactionRepository: txRepo, YahooService: yahooSvc}
}

// convertStocksToDTO converts model stocks to DTO format
func (s *StocksService) convertStocksToDTO(stocks []models.StockHolding) []dto.StocksData {
	stocksData := make([]dto.StocksData, 0, len(stocks))
	for _, stock := range stocks {
		stocksData = append(stocksData, dto.StocksData{
			Id:         stock.Id,
			Ticker:     stock.Ticker,
			Sector:     stock.Sector,
			Lot:        stock.Lot,
			AvgPrice:   stock.AvgPrice,
			BrokerID:   stock.BrokerID,
			BrokerName: stock.BrokerName,
			CreatedAt:  stock.CreatedAt,
			UpdatedAt:  stock.UpdatedAt,
		})
	}
	return stocksData
}

// syncMissingSectors fetches and updates missing sector information from Yahoo service
func (s *StocksService) syncMissingSectors(stocks []models.StockHolding) {
	if s.YahooService == nil || len(stocks) == 0 {
		return
	}

	missingTickers := make([]string, 0)
	for _, stock := range stocks {
		if stock.Sector == "" && stock.Ticker != "" {
			missingTickers = append(missingTickers, stock.Ticker)
		}
	}

	if len(missingTickers) == 0 {
		return
	}

	sectors, err := s.YahooService.GetSectors(missingTickers)
	if err != nil || len(sectors) == 0 {
		return
	}

	for i := range stocks {
		if stocks[i].Sector == "" {
			if sec, ok := sectors[stocks[i].Ticker]; ok && sec != "" {
				stocks[i].Sector = sec
				_ = s.StocksRepository.UpdateSectorByTicker(stocks[i].Ticker, sec)
			}
		}
	}
}

func (s *StocksService) GetAllStocks(userID uint, limiter int) (*dto.StocksResponse, error) {
	stocks, err := s.StocksRepository.GetAllStocks(userID, limiter)
	if err != nil {
		return nil, errs.InternalServerError("Failed to retrieve stock data")
	}

	s.syncMissingSectors(stocks)

	stocksData := s.convertStocksToDTO(stocks)

	response := &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received all stocks details",
		StocksData: stocksData,
	}
	return response, nil
}

func (s *StocksService) GetStocks(userID uint, stockTicker []string) (*dto.StocksResponse, error) {
	stocks, err := s.StocksRepository.GetStocks(userID, stockTicker)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.NotFound("Stock not found")
		}
		return nil, errs.InternalServerError("Failed to retrieve stock data")
	}

	if len(stocks) == 0 {
		return nil, errs.NotFound("Stock not found")
	}

	s.syncMissingSectors(stocks)

	stocksData := s.convertStocksToDTO(stocks)

	response := &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Success received stocks details",
		StocksData: stocksData,
	}
	return response, nil
}

func (s *StocksService) AddStockData(userID uint, payload *dto.StocksRequest) (*dto.StocksResponse, error) {
	stock := &dto.StocksData{
		Ticker:     strings.ToUpper(strings.TrimSpace(payload.Ticker)),
		Sector:     strings.TrimSpace(payload.Sector),
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

	if stock.Sector == "" && s.YahooService != nil {
		if sec, err := s.YahooService.GetSector(stock.Ticker); err == nil && sec != "" {
			stock.Sector = sec
		}
	}

	existing, err := s.StocksRepository.GetStockByTickerAndBroker(userID, stock.Ticker, stock.BrokerID)
	if err == nil {
		if existing.Lot > 0 {
			return nil, errs.BadRequest("stock already exists for this broker, use PUT /stock/:ticker or POST /transaction")
		}
		existing.Lot = stock.Lot
		existing.AvgPrice = stock.AvgPrice
		if stock.Sector != "" {
			existing.Sector = stock.Sector
		}
		if stock.BrokerName != "" {
			existing.BrokerName = stock.BrokerName
		}
		updatedStock, err := s.StocksRepository.UpdateStockData(userID, []string{stock.Ticker}, existing)
		if err != nil {
			return nil, errs.InternalServerError("Failed to reactivate stock data")
		}
		return &dto.StocksResponse{
			StatusCode: http.StatusCreated,
			Message:    "Stock data added successfully",
			StocksData: []dto.StocksData{{
				Id:         updatedStock.Id,
				Ticker:     updatedStock.Ticker,
				Sector:     updatedStock.Sector,
				Lot:        updatedStock.Lot,
				AvgPrice:   updatedStock.AvgPrice,
				BrokerID:   updatedStock.BrokerID,
				BrokerName: updatedStock.BrokerName,
				CreatedAt:  updatedStock.CreatedAt,
				UpdatedAt:  updatedStock.UpdatedAt,
			}},
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.InternalServerError("Failed to validate existing stock")
	}

	addedStock, err := s.StocksRepository.AddStockData(&models.StockHolding{
		UserID:     userID,
		Ticker:     stock.Ticker,
		Sector:     stock.Sector,
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
			Id:         addedStock.Id,
			Ticker:     addedStock.Ticker,
			Sector:     addedStock.Sector,
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

func (s *StocksService) UpdateStock(userID uint, stockTicker []string, payload *dto.StocksRequest) (*dto.StocksResponse, error) {
	stocks, err := s.StocksRepository.GetStocksReturnModels(userID, stockTicker)
	if err != nil {
		return nil, err
	}

	if stocks == nil {
		return nil, errs.NotFound("Stock not found")
	}

	if payload.Ticker != "" {
		stocks.Ticker = payload.Ticker
	}

	if payload.Lot != 0 {
		stocks.Lot = payload.Lot
	}

	if payload.AvgPrice != 0 {
		stocks.AvgPrice = payload.AvgPrice
	}

	if payload.BrokerID != 0 {
		stocks.BrokerID = payload.BrokerID
	}

	if payload.BrokerName != "" {
		stocks.BrokerName = payload.BrokerName
	}

	if payload.Sector != "" {
		stocks.Sector = payload.Sector
	}

	updatedStock, err := s.StocksRepository.UpdateStockData(userID, stockTicker, &models.StockHolding{
		Ticker:     stocks.Ticker,
		Sector:     stocks.Sector,
		Lot:        stocks.Lot,
		AvgPrice:   stocks.AvgPrice,
		BrokerID:   stocks.BrokerID,
		BrokerName: stocks.BrokerName,
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
			Sector:     updatedStock.Sector,
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

func (s *StocksService) DeleteStockData(userID uint, stockTicker []string) (*dto.StocksResponse, error) {
	for _, ticker := range stockTicker {
		normalized := strings.ToUpper(strings.TrimSpace(ticker))
		if normalized == "" {
			continue
		}
		count, err := s.TransactionRepository.CountTransactionsByTicker(userID, normalized)
		if err != nil {
			return nil, errs.InternalServerError("Failed to validate transaction history")
		}
		if count > 0 {
			return nil, errs.BadRequest("cannot delete stock with transaction history; keep holding or offset via SELL transaction")
		}
	}

	err := s.StocksRepository.DeleteStockData(userID, stockTicker)
	if err != nil {
		return nil, errs.InternalServerError("Failed to delete stock data")
	}

	response := &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Stock data deleted successfully",
	}
	return response, nil
}

func (s *StocksService) SyncStockSectors(userID uint) (*dto.StocksResponse, error) {
	stocks, err := s.StocksRepository.GetAllStocks(userID, 0)
	if err != nil {
		return nil, errs.InternalServerError("Failed to retrieve stock data")
	}

	s.syncMissingSectors(stocks)

	stocksData := s.convertStocksToDTO(stocks)

	return &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Success synced all stock sectors",
		StocksData: stocksData,
	}, nil
}
