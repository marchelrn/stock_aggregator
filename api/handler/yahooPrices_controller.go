package handler

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type YahooPricesController struct {
	yahooService contract.YahooService
}

func (c *YahooPricesController) InitService(s *contract.Service) {
	c.yahooService = s.Yahoo
}

func (c *YahooPricesController) GetPrice(ctx *gin.Context) {
	ticker := strings.ToUpper(ctx.Param("ticker"))

	price, err := c.yahooService.GetPrice(ticker)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": price,
	})
}

func (c *YahooPricesController) GetAllCachedPrices(ctx *gin.Context) {
	prices, err := c.yahooService.GetAllPrices()
	if err != nil {
		HandleError(ctx, errs.InternalServerError(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": prices,
	})
}

func (c *YahooPricesController) GetPrices(ctx *gin.Context) {
	tickers := ctx.Query("ticker")

	if tickers == "" {
		HandleError(ctx, errs.BadRequest("ticker is required (e.g ?Ticker=ASII,BBCA)"))
		return
	}

	tickerList := strings.Split(strings.ToUpper(tickers), ",")

	prices, failed, err := c.yahooService.GetPrices(tickerList)
	if err != nil {
		HandleError(ctx, errs.BadRequest(err.Error()))
		return
	}

	// `failed` berisi ticker yang tidak berhasil diambil (jika ada), agar client
	// tahu bahwa hasil ini sebagian dan bisa menandai ticker yang salah.
	notFound := make([]string, 0, len(failed))
	for t := range failed {
		notFound = append(notFound, t)
	}
	sort.Strings(notFound)

	ctx.JSON(http.StatusOK, gin.H{
		"data":      prices,
		"not_found": notFound,
		"partial":   len(notFound) > 0,
	})
}
