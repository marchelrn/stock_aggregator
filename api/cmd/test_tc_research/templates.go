package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const templateReadTimeout = 5 * time.Second

type templateIdentity struct {
	Key     string `json:"key"`
	Version int    `json:"version"`
	Source  string `json:"source"`
}

// A snapshot keeps all cases on the same configuration, even during DB activation changes.
type templateSnapshot []models.PDFParsingTemplate

func (s templateSnapshot) ListActive(ctx context.Context) ([]models.PDFParsingTemplate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s, nil
}

type databaseTemplateLoader func(context.Context) ([]models.PDFParsingTemplate, error)

func loadTemplates(ctx context.Context, o options, loadDB databaseTemplateLoader) (templateSnapshot, error) {
	if o.configFile != "" {
		data, err := os.ReadFile(o.configFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read preview config file")
		}
		if !json.Valid(data) {
			return nil, fmt.Errorf("invalid preview config JSON")
		}
		broker := o.brokerName
		if broker == "" {
			broker = "Preview broker"
		}
		return templateSnapshot{{Key: "preview", BrokerName: broker, Version: 1, Active: true, Config: data}}, nil
	}
	templates, err := loadDB(ctx)
	if err != nil {
		// Driver/repository errors can contain credentials or private configuration.
		return nil, fmt.Errorf("cannot load active PDF templates from database; check DB configuration, connectivity and applied create_pdf_parsing_templates migration")
	}
	var active templateSnapshot
	for _, template := range templates {
		if template.Active {
			active = append(active, template)
		}
	}
	if len(active) == 0 {
		return nil, fmt.Errorf("database has no active PDF templates; apply migration and check activation")
	}
	return active, nil
}

func loadDatabaseTemplates(ctx context.Context) ([]models.PDFParsingTemplate, error) {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("cannot load .env")
	}
	// Match config.Production's environment selection without its fatal logging.
	var dsn string
	if os.Getenv("ENV") == "production" {
		dsn = os.Getenv("DB_URL")
		if dsn == "" {
			return nil, fmt.Errorf("missing database configuration")
		}
	} else {
		for _, key := range []string{"DB_USER", "DB_HOST", "DB_PORT", "DB_NAME"} {
			if strings.TrimSpace(os.Getenv(key)) == "" {
				return nil, fmt.Errorf("missing database configuration")
			}
		}
		dsn = config.LocalDb()
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database configuration")
	}
	cfg.ConnectTimeout = templateReadTimeout
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.RuntimeParams["statement_timeout"] = "5000"
	sqlDB := stdlib.OpenDB(*cfg)
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database connection failed")
	}
	// Do not use database.ConnectDB/server startup: they log private errors or run migrations.
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), DisableAutomaticPing: true, SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot initialize readonly database client")
	}
	return repository.ImplPDFTemplateRepository(db).ListActive(ctx)
}
