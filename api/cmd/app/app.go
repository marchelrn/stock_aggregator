package app

import (
	"log"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/internal/server"
)

func Run() {
	log.Println("Server started")
	config.Load()
	server.Run()
}
