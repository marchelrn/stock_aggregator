package handler

import (
	"github.com/marchelrn/stock_api/contract"

	"github.com/gin-gonic/gin"
)

type Controller interface {
	InitService(s *contract.Service)
}

func New(app *gin.Engine, service *contract.Service)  {
	allControllers := []Controller{
		&StocksController{},
		&BrokersController{},

	}

	for _, ctrl := range allControllers {
		ctrl.InitService(service)
	}
}