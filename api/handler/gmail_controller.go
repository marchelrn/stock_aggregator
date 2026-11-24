package handler

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type GmailController struct {
	service *contract.Service
	jobsMu  sync.Mutex
	jobs    map[string]*setupJob
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

	// Check if full sync is requested
	fullSync := ctx.Query("full") == "true"

	var response interface{}
	var err error

	if fullSync {
		response, err = c.service.Gmail.SyncTradeConfirmationsFullContext(ctx.Request.Context(), uint(uid), "")
	} else {
		response, err = c.service.Gmail.SyncTradeConfirmations(uint(uid))
	}

	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// SyncSetupHistorical menjalankan sync historis setelah user menyimpan broker
// pada setup awal. Transaksi saham diambil penuh (tanpa checkpoint), lalu
// mutasi kas disusul. Kegagalan salah satu tahap dilaporkan terpisah agar UI
// bisa menampilkan langkah mana yang gagal.
func (c *GmailController) SyncSetupHistorical(ctx *gin.Context) {
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

	var request struct {
		BrokerName string `json:"broker_name"`
	}
	if ctx.ShouldBindJSON(&request) != nil || strings.TrimSpace(request.BrokerName) == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Pilih broker yang ingin disinkronkan."})
		return
	}
	broker := strings.TrimSpace(request.BrokerName)
	if ctx.Query("async") == "1" {
		c.startSetupJob(ctx, uint(uid), broker)
		return
	}
	result, status := c.runSetup(ctx.Request.Context(), uint(uid), broker, false)
	ctx.JSON(status, result)
}

func (c *GmailController) runSetup(ctx context.Context, uid uint, broker string, summaryOnly bool) (gin.H, int) {
	response := gin.H{}
	trade, tradeErr := c.service.Gmail.SyncTradeConfirmationsFullContext(ctx, uid, broker)
	if tradeErr != nil {
		response["trade_error"] = "Transaksi saham: " + tradeErr.Error()
	} else {
		if summaryOnly && trade != nil {
			trade.ParsedTrades = nil
		}
		response["trade_sync"] = trade
		if trade != nil && len(trade.PendingPDFs) > 0 {
			response["trade_warning"] = trade.Message
		}
	}
	cash, cashErr := c.service.Gmail.SyncCashMutationsContext(ctx, uid, broker)
	if cashErr != nil {
		response["cash_error"] = "Mutasi kas: " + cashErr.Error()
	} else {
		if summaryOnly && cash != nil {
			cash.ParsedMutations = nil
		}
		response["cash_sync"] = cash
	}
	if tradeErr != nil && cashErr != nil {
		return response, errs.GetStatusCode(tradeErr)
	}
	return response, http.StatusOK
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

// SyncCashMutations triggers a manual sync of cash deposit/withdrawal emails from Gmail
func (c *GmailController) SyncCashMutations(ctx *gin.Context) {
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

	response, err := c.service.Gmail.SyncCashMutations(uint(uid))
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// GetCashMutations returns all stored cash mutations for the authenticated user
func (c *GmailController) GetCashMutations(ctx *gin.Context) {
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

	response, err := c.service.Gmail.GetCashMutations(uint(uid))
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}
