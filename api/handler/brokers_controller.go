package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type BrokersController struct {
	service *contract.Service
}

func (c *BrokersController) InitService(svc *contract.Service) {
	c.service = svc
}

func (c *BrokersController) GetBroker(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	idParam := ctx.Param("id")
	var id int
	_, err = fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid broker ID"})
		return
	}

	broker, err := c.service.Brokers.GetBroker(userID, id)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Success received Broker",
		"data":    broker,
	})
}

func (c *BrokersController) GetAllBrokers(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	allBrokers, err := c.service.Brokers.GetAllBrokers(userID)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Success received all Brokers",
		"data":    allBrokers,
	})
}

func (c *BrokersController) GetBrokersDetails(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	brokerName := ctx.Param("name")
	brokerNames := strings.Split(brokerName, ",")

	brokerDetails, err := c.service.Brokers.GetBrokersDetails(userID, brokerNames)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": brokerDetails.Message,
		"data":    brokerDetails.BrokersData,
	})
}

func (c *BrokersController) GetAllBrokersDetails(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	brokers, err := c.service.Brokers.GetAllBrokersDetails(userID)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Success received all Brokers",
		"data":    brokers,
	})
}

func (c *BrokersController) AddBrokerData(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	var payload dto.BrokerRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := c.service.Brokers.AddBrokerData(userID, &payload)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"data":    response.BrokersData,
	})
}

func (c *BrokersController) DeleteBrokerData(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	brokerName := ctx.Param("name")
	brokerNames := strings.Split(brokerName, ",")

	response, err := c.service.Brokers.DeleteBrokerData(userID, brokerNames)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"status":  response.StatusCode,
	})
}

func (c *BrokersController) UpdateBrokerData(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	idParam := ctx.Param("id")
	var id int
	_, err = fmt.Sscanf(idParam, "%d", &id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid broker ID"})
		return
	}

	var payload dto.BrokerRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := c.service.Brokers.UpdateBrokerData(userID, id, &payload)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"status":  response.StatusCode,
	})
}

func (c *BrokersController) TransferCashBetweenBrokers(ctx *gin.Context) {
	userID, err := getUserID(ctx)
	if err != nil {
		HandleError(ctx, err)
		return
	}

	var payload dto.TransferCashRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := c.service.Brokers.TransferCashBetweenBrokers(userID, &payload)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": response.Message,
		"status":  response.StatusCode,
	})
}
