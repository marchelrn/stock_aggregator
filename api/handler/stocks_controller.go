package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type StocksController struct {
	service contract.StocksService
}

func (s *StocksController) InitService(svc *contract.Service) {
	if svc == nil {
		return
	}
	s.service = svc.Stocks
}

// parseTickers extracts and deduplicates tickers from comma-separated string
func (s *StocksController) parseTickers(tickerParam string) ([]string, error) {
	if tickerParam == "" {
		return nil, errs.BadRequest("Stock ticker is required")
	}

	rawTickers := strings.Split(strings.ToUpper(tickerParam), ",")
	tickers := make([]string, 0, len(rawTickers))
	seen := make(map[string]struct{}, len(rawTickers))

	for _, ticker := range rawTickers {
		ticker = strings.TrimSpace(ticker)
		if ticker == "" {
			continue
		}
		if _, exists := seen[ticker]; exists {
			continue
		}
		seen[ticker] = struct{}{}
		tickers = append(tickers, ticker)
	}

	if len(tickers) == 0 {
		return nil, errs.BadRequest("Stock ticker is required")
	}

	return tickers, nil
}

func (s *StocksController) GetAllStocks(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	limiterParam := c.DefaultQuery("limit", "10")
	limiter, err := strconv.Atoi(limiterParam)
	if err != nil {
		HandleError(c, errs.BadRequest("Invalid limit parameter"))
		return
	}

	response, err := s.service.GetAllStocks(userID, limiter)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"data":    response.StocksData,
	})
}

func (s *StocksController) GetStocks(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	stockTickerParam := c.Param("ticker")
	tickers, err := s.parseTickers(stockTickerParam)
	if err != nil {
		HandleError(c, err)
		return
	}

	response, err := s.service.GetStocks(userID, tickers)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"data":    response.StocksData,
	})
}

func (s *StocksController) AddStockData(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	var payload dto.StocksRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		HandleError(c, errs.BadRequest("Invalid request payload"))
		return
	}

	response, err := s.service.AddStockData(userID, &payload)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"data":    response.StocksData,
	})
}

func (s *StocksController) UpdateStock(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	stockTickerParam := c.Param("ticker")
	tickers, err := s.parseTickers(stockTickerParam)
	if err != nil {
		HandleError(c, err)
		return
	}

	var payload dto.StocksRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		HandleError(c, errs.BadRequest("Invalid request payload"))
		return
	}

	response, err := s.service.UpdateStock(userID, tickers, &payload)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"data":    response.StocksData,
	})
}

func (s *StocksController) DeleteStockData(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	tickerParam := c.Param("ticker")
	tickers, err := s.parseTickers(tickerParam)
	if err != nil {
		HandleError(c, err)
		return
	}

	response, err := s.service.DeleteStockData(userID, tickers)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"status":  response.StatusCode,
	})
}

func (s *StocksController) SyncStockSectors(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		HandleError(c, err)
		return
	}

	response, err := s.service.SyncStockSectors(userID)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"data":    response.StocksData,
	})
}
