package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type TransactionController struct {
	service contract.TransactionService
}

func (c *TransactionController) InitService(s *contract.Service) {
	c.service = s.Transaction
}

func (c *TransactionController) GetTransaction(ctx *gin.Context) {
	idParam := ctx.Param("id")
	if idParam == "" {
		HandleError(ctx, errs.BadRequest("transaction ID is required"))
		return
	}

	id, err := strconv.Atoi(idParam)
	if err != nil {
		HandleError(ctx, errs.BadRequest("invalid transaction ID"))
		return
	}

	response, err := c.service.GetTransaction(id)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(response.StatusCode, response)
}

func (c *TransactionController) GetAllTransactions(ctx *gin.Context) {
	response, err := c.service.GetAllTransactions()
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(response.StatusCode, response)
}

func (c *TransactionController) CreateTransaction(ctx *gin.Context) {
	var payload dto.TransactionRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		HandleError(ctx, errs.BadRequest("invalid request payload"))
		return
	}

	ticker := ctx.QueryArray("ticker")

	response, err := c.service.CreateTransaction(ticker, &payload)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(response.StatusCode, response)
}

func (c *TransactionController) DeleteTransaction(ctx *gin.Context) {
	idParam := ctx.Param("id")
	if idParam == "" {
		HandleError(ctx, errs.BadRequest("transaction ID is required"))
		return
	}

	id, err := strconv.Atoi(idParam)
	if err != nil {
		HandleError(ctx, errs.BadRequest("invalid transaction ID"))
		return
	}

	response, err := c.service.DeleteTransaction(id)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(response.StatusCode, response)
}
