package database

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/marchelrn/stock_api/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ConnectDB() (*gorm.DB, *sql.DB) {
	cfg := config.GetConfig()
	var isProd bool
	var loggerLevel logger.LogLevel

	if cfg.IsProd == false {
		loggerLevel = logger.Info
	} else {
		loggerLevel = logger.Error
	}

	if cfg.IsProd == true {
		isProd = true
	} else {
		isProd = false
	}

	sqllogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  loggerLevel, // Development mode : Debug, Production mode : Info
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      isProd, // Development mode : False
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(cfg.DBUrl), &gorm.Config{
		Logger:                 sqllogger,
		SkipDefaultTransaction: true,
		AllowGlobalUpdate:      false,
	})

	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}
	log.Println("Success Connect to Database")

	log.Println("Set database configuration")
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get database instance: ", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, sqlDB
}
