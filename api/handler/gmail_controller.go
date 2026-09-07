package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type GmailController struct {
	service *contract.Service
}

func (c *GmailController) InitService(svc *contract.Service) {
	c.service = svc
}

// SyncTradeConfirmations triggers a manual sync of trade confirmation emails from Gmail
func (c *GmailController) SyncTradeConfirmations(ctx *gin.Context) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Convert user_id to uint
	uid, ok := userID.(float64)
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	response, err := c.service.Gmail.SyncTradeConfirmations(uint(uid))
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// GetSyncStatus returns the current Gmail sync status for the authenticated user
func (c *GmailController) GetSyncStatus(ctx *gin.Context) {
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid, ok := userID.(float64)
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	response, err := c.service.Gmail.GetSyncStatus(uint(uid))
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}
