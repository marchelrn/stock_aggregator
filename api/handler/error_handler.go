package handler

import (
	errs "github.com/marchelrn/stock_api/pkg/error"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func HandleError(c *gin.Context, err error) {
	statusCode := errs.GetStatusCode(err)
	c.JSON(statusCode, ErrorResponse{
		StatusCode: statusCode,
		Message:    err.Error(),
	})
}
