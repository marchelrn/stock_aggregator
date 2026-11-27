package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
)

func PortfolioPerformance(repo contract.PortfolioPerformanceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := getUserID(c)
		if err != nil {
			HandleError(c, err)
			return
		}
		wita := time.FixedZone("WITA", 8*60*60)
		since := time.Now().In(wita).AddDate(0, 0, -90).Format("2006-01-02")
		points, err := repo.List(c.Request.Context(), userID, since)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Riwayat kinerja belum dapat dimuat."})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"data": points})
	}
}
