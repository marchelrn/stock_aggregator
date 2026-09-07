package routes

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/handler"
	"github.com/marchelrn/stock_api/middleware"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func SetupRoutes(s *contract.Service) *gin.Engine {
	r := gin.Default()
	r.RedirectTrailingSlash = false

	cfg := config.GetConfig()

	var limitter int64
	if cfg.IsProd == false {
		limitter = 1000
	} else {
		limitter = 100
	}

	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  limitter,
	}

	store := memory.NewStore()
	instance := limiter.New(store, rate)
	rateLimitter := mgin.NewMiddleware(instance)
	r.Use(rateLimitter)

	defaultConfig := cors.DefaultConfig()
	defaultConfig.AllowAllOrigins = true
	// defaultConfig.AllowOrigins = []string{"http://localhost:3000", "http://localhost:5500"}
	defaultConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	defaultConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "Accept", "X-Requested-With"}
	defaultConfig.AllowCredentials = true
	defaultConfig.ExposeHeaders = []string{"Content-Length"}
	r.Use(cors.New(defaultConfig))

	stockController := &handler.StocksController{}
	stockController.InitService(s)

	brokerController := &handler.BrokersController{}
	brokerController.InitService(s)

	transactionController := &handler.TransactionController{}
	transactionController.InitService(s)

	yahooController := &handler.YahooPricesController{}
	yahooController.InitService(s)

	authController := &handler.AuthController{}
	authController.InitService(s)

	gmailController := &handler.GmailController{}
	gmailController.InitService(s)

	api := r.Group("/")
	{
		api.GET("/health", func(ctx *gin.Context) {
			ctx.JSON(200, gin.H{
				"status": "api is healthy",
			})
		})
		// Stock routes
		api.GET("/stocks", stockController.GetAllStocks)
		api.GET("/stock/:ticker", stockController.GetStocks)
		api.POST("/stock", stockController.AddStockData)
		api.PUT("/stock/:ticker", stockController.UpdateStock)
		api.DELETE("/stock/:ticker", stockController.DeleteStockData)

		// Broker routes
		api.GET("/brokers", brokerController.GetAllBrokers)
		api.GET("/brokers-details", brokerController.GetAllBrokersDetails)
		api.GET("/broker/:name", brokerController.GetBrokersDetails)
		api.POST("/broker", brokerController.AddBrokerData)
		api.PUT("/broker/:id", brokerController.UpdateBrokerData)
		api.DELETE("/broker/:name", brokerController.DeleteBrokerData)
		api.POST("/broker/transfer-cash", brokerController.TransferCashBetweenBrokers)

		// Transaction routes
		api.GET("/transaction/:id", transactionController.GetTransaction)  // Get a specific transaction by ID
		api.GET("/transactions", transactionController.GetAllTransactions) // Get all transactions
		api.POST("/transaction", transactionController.CreateTransaction)
		api.DELETE("/transaction/:id", transactionController.DeleteTransaction)

		// Yahoo Finance routes
		api.GET("/price/:ticker", yahooController.GetPrice)
		api.GET("/prices", yahooController.GetPrices)
		api.GET("/prices/cache", yahooController.GetAllCachedPrices)

		// Google OAuth routes
		api.POST("/auth/register", authController.Register)
		api.POST("/auth/login", authController.Login)
		api.GET("/auth/google/login", authController.GoogleLogin)
		api.GET("/auth/google/callback", authController.GoogleCallback)
	}

	// Protected routes (requires JWT authentication)
	protected := r.Group("/api")
	protected.Use(middleware.JWTAuth())
	{
		// Gmail sync routes
		protected.POST("/gmail/sync", gmailController.SyncTradeConfirmations)
		protected.GET("/gmail/status", gmailController.GetSyncStatus)
	}

	return r
}
