package migrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/marchelrn/stock_api/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type AddTrimegahTemplateMigration struct{}

func getAddTrimegahTemplate() *AddTrimegahTemplateMigration {
	return &AddTrimegahTemplateMigration{}
}

func (*AddTrimegahTemplateMigration) Name() string {
	return "add_trimegah_template"
}

func (*AddTrimegahTemplateMigration) SkipProd() bool {
	return false
}

func (m *AddTrimegahTemplateMigration) Up(conn *sql.Tx) error {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("wrap tx in gorm: %w", err)
	}
	return addTrimegahTemplate(db)
}

func (*AddTrimegahTemplateMigration) Down(conn *sql.Tx) error {
	_, err := conn.Exec("DELETE FROM pdf_parsing_templates WHERE key = 'trimegah' AND version = 1")
	return err
}

// addTrimegahTemplate seed template Trimegah agar TC-nya bisa ditemukan Gmail sync.
func addTrimegahTemplate(db *gorm.DB) error {
	// Cek apakah template Trimegah sudah ada
	var existing models.PDFParsingTemplate
	err := db.Where("key = ? AND version = ?", "trimegah", 1).First(&existing).Error
	if err == nil {
		// Template sudah ada, skip
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("check existing Trimegah template: %w", err)
	}

	// Buat config minimal untuk Trimegah (fallback ke hardcoded parser)
	config := map[string]interface{}{
		"schema_version":   1,
		"required_markers": []string{"TRIMEGAH"},
		"note":             "Trimegah TC parsed by hardcoded TrimegahParser; this template exists for Gmail discovery only",
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal Trimegah config: %w", err)
	}

	template := models.PDFParsingTemplate{
		Key:             "trimegah",
		BrokerName:      "Trimegah Sekuritas",
		Version:         1,
		Active:          true,
		Config:          configJSON,
		EmailSender:     "trim.operation@trimegah.com",
		SubjectContains: "TC",
		FilenameSuffix:  "",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := db.Create(&template).Error; err != nil {
		return fmt.Errorf("create Trimegah template: %w", err)
	}

	return nil
}
