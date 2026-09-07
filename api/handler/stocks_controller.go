package handler

import (
	"fmt"
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
	fmt.Println("DEBUG: Initializing StocksController with StocksService")
	if svc == nil {
		fmt.Println("ERROR: Provided service is nil")
		return
	}
	s.service = svc.Stocks
}

func (s *StocksController) GetAllStocks(c *gin.Context) {
	limiterParam := c.DefaultQuery("limit", "10")
	limiter, err := strconv.Atoi(limiterParam)
	if err != nil {
		HandleError(c, errs.BadRequest("Invalid limit parameter"))
		return
	}

	response, err := s.service.GetAllStocks(limiter)
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
	stockTickerParam := c.Param("ticker")
	if stockTickerParam == "" {
		HandleError(c, errs.BadRequest("Stock ticker is required"))
		return
	}

	rawTickers := strings.Split(strings.ToUpper(stockTickerParam), ",")
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
		HandleError(c, errs.BadRequest("Stock ticker is required"))
		return
	}

	response, err := s.service.GetStocks(tickers)
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
	var payload dto.StocksRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		HandleError(c, errs.BadRequest("Invalid request payload"))
		return
	}

	response, err := s.service.AddStockData(&payload)
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
	stockTickerParam := c.Param("ticker")
	if stockTickerParam == "" {
		HandleError(c, errs.BadRequest("Stock ticker is required"))
		return
	}
	rawTickers := strings.Split(strings.ToUpper(stockTickerParam), ",")
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
		HandleError(c, errs.BadRequest("Stock ticker is required"))
		return
	}

	var payload dto.StocksRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		HandleError(c, errs.BadRequest("Invalid request payload"))
		return
	}

	response, err := s.service.UpdateStock(tickers, &payload)
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
	tickerParam := c.Param("ticker")
	if tickerParam == "" {
		HandleError(c, errs.BadRequest("Stock ticker is required"))
		return
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
		HandleError(c, errs.BadRequest("Stock ticker is required"))
		return
	}

	response, err := s.service.DeleteStockData(tickers)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"status":  response.StatusCode,
	})
}
