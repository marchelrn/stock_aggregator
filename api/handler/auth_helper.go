package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

// getUserID extracts the authenticated user's ID from the Gin context (set by JWTAuth middleware)
func getUserID(c *gin.Context) (uint, error) {
	val, exists := c.Get("user_id")
	if !exists {
		return 0, errs.Unauthorized("User belum terautentikasi")
	}

	switch v := val.(type) {
	case float64:
		return uint(v), nil
	case uint:
		return v, nil
	case int:
		return uint(v), nil
	case int64:
		return uint(v), nil
	case string:
		id, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0, errs.Unauthorized("Format user_id tidak valid")
		}
		return uint(id), nil
	default:
		return 0, errs.Unauthorized("Format user_id tidak valid")
	}
}
