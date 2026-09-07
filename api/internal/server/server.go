package server

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/internal/database"
	"github.com/marchelrn/stock_api/migrations"
	"github.com/marchelrn/stock_api/repository"
	"github.com/marchelrn/stock_api/routes"
	"github.com/marchelrn/stock_api/service"
)

func Run() {
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetOutput(os.Stdout)

	cfg := config.GetConfig()

	db, sqlDB := database.ConnectDB()
	
	repo := repository.New(db)
	srv, err := service.New(repo)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	r:= routes.SetupRoutes(srv)

	migrations.Up(sqlDB)

	serv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: r,
	}

	log.Printf("Server is running on port %s\n", cfg.Port)
	log.Fatal(serv.ListenAndServe())
}
