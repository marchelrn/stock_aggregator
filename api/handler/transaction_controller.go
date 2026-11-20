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
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

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

	response, err := c.service.GetTransaction(userID, id)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(response.StatusCode, response)
}

func (c *TransactionController) GetAllTransactions(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	response, err := c.service.GetAllTransactions(userID)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(response.StatusCode, response)
}

func (c *TransactionController) CreateTransaction(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	var payload dto.TransactionRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		HandleError(ctx, errs.BadRequest("invalid request payload"))
		return
	}

	ticker := ctx.QueryArray("ticker")

	response, err := c.service.CreateTransaction(userID, ticker, &payload)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(response.StatusCode, response)
}

func (c *TransactionController) DeleteTransaction(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

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

	response, err := c.service.DeleteTransaction(userID, id)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	ctx.JSON(response.StatusCode, response)
}

func (c *TransactionController) History(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		HandleError(ctx, errs.BadRequest("invalid page"))
		return
	}
	size, err := strconv.Atoi(ctx.DefaultQuery("page_size", "5"))
	if err != nil || size < 1 || size > 50 {
		HandleError(ctx, errs.BadRequest("invalid page size"))
		return
	}
	result, err := c.service.History(ctx.Request.Context(), userID, page, size, ctx.Query("q"))
	if err != nil {
		HandleError(ctx, errs.InternalServerError("failed to load history"))
		return
	}
	ctx.JSON(200, result)
}
