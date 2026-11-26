package service

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/migrations"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/repository"
	"github.com/marchelrn/stock_api/service/parser"
	"google.golang.org/api/gmail/v1"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDoclingAjaibPersistence(t *testing.T) {
	dsn, path := os.Getenv("DOCLING_TEST_DSN"), os.Getenv("AJAIB_SAMPLE_PDF")
	legacy := os.Getenv("AJAIB_DOCLING_ACCOUNTING_PDF")
	if dsn == "" || (path == "" && legacy == "") {
		t.Skip("requires disposable PostgreSQL and an Ajaib PDF")
	}
	t.Setenv("TC_FALLBACK_PROVIDER", "docling")
	config.Load()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	migrations.Up(sqlDB)
	tests := []struct {
		name, path string
		valid      bool
	}{
		{"original", path, true},
		{"legacy_buy", legacy, true},
		{"reordered", os.Getenv("AJAIB_DOCLING_VARIANT"), true},
		{"bad_tax", os.Getenv("AJAIB_DOCLING_INVALID"), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.path == "" {
				t.Skip("requires optional variant")
			}
			testDoclingTransaction(t, db, test.path, test.valid, test.name != "original" && test.name != "legacy_buy", test.name == "legacy_buy")
		})
	}
}
func testDoclingTransaction(t *testing.T, db *gorm.DB, path string, valid, gmailPath, legacyBuy bool) {
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	repository := repository.New(tx)
	svc := GmailServiceImpl{Reviews: repository.Reviews}
	user := models.Users{Email: "docling-isolated@example.test", CreatedAt: time.Now()}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	broker, err := repository.Brokers.AddBrokerData(&models.Broker{UserID: user.Id, Name: ajaibBroker})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var review *models.TradeReview
	var saved, found int
	sentAt := time.Date(2026, 10, 18, 13, 14, 15, 0, time.UTC)
	if gmailPath {
		services, initErr := New(repository)
		if initErr != nil {
			t.Fatal(initErr)
		}
		svc := services.Gmail.(*GmailServiceImpl)
		ctx := parser.WithBrokerScope(t.Context(), map[string]int{"ajaib sekuritas": broker.ID})
		msg := &gmail.Message{Id: "docling-test-mail", InternalDate: sentAt.UnixMilli(), Payload: &gmail.MessagePart{
			Filename: "/home/lerch/Downloads/TC_20240912_O0CP1B.pdf", PartId: "1", MimeType: "application/pdf", Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString(pdf)},
			Headers: []*gmail.MessagePartHeader{{Name: "From", Value: "noreply@ajaib.co.id"}, {Name: "Subject", Value: "Rangkuman Transaksi Harian"}},
		}}
		local, pending, parseErr := svc.processSyncAttachments(ctx, nil, msg, user.Id, map[string][]models.TradeReview{}, map[uint]dto.PendingTradePDF{}, &found, &saved, new(int))
		if parseErr != nil || len(local) != 0 {
			t.Fatalf("attachment path must use fallback: local=%d err=%v", len(local), parseErr)
		}
		review = &models.TradeReview{}
		if queryErr := tx.Where("user_id=? AND broker_id=?", user.Id, broker.ID).First(review).Error; queryErr != nil {
			t.Fatal(queryErr)
		}
		if len(pending) > 0 {
			err = errors.New(pending[0].Reason)
		}
	} else {
		review, err = repository.Reviews.Remember(t.Context(), &models.TradeReview{UserID: user.Id, BrokerID: broker.ID, BrokerName: broker.Name, Digest: "docling-integration", Filename: "sample.pdf", State: "pending"})
		if err != nil {
			t.Fatal(err)
		}
		saved, found, err = svc.syncFallbackPDF(t.Context(), user.Id, review.ID, pdf, broker.Name, sentAt)
	}

	if !valid {
		if err == nil || saved != 0 || found != 0 || !strings.Contains(err.Error(), "Total SELL tidak cocok") {
			t.Fatalf("expected financial validation rejection; saved=%d found=%d err=%v", saved, found, err)
		}
		var count int64
		if err := tx.Model(&models.Transaction{}).Where("user_id=?", user.Id).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("invalid PDF inserted transactions", err)
		}
		current, err := repository.Reviews.Get(t.Context(), user.Id, review.ID)
		if err != nil || current.State != "pending" {
			t.Fatal("invalid PDF was not left pending", err)
		}
		return
	}
	if err != nil || saved != 1 || found != 1 {
		t.Fatalf("saved=%d found=%d err=%v", saved, found, err)
	}
	var rows []models.Transaction
	if err := tx.Where("user_id=? AND broker_id=?", user.Id, broker.ID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one saved trade, got %d", len(rows))
	}
	row := rows[0]
	if row.EmailSentAt == nil || !row.EmailSentAt.Equal(sentAt) {
		t.Fatal("Gmail timestamp not stored with fallback transaction")
	}
	if err := tx.Model(&row).Update("email_sent_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.Reviews.RecordEmailSentAt(t.Context(), user.Id, review.ID, sentAt); err != nil {
		t.Fatal(err)
	}
	if err := tx.First(&row, row.Id).Error; err != nil || row.EmailSentAt == nil || !row.EmailSentAt.Equal(sentAt) {
		t.Fatal("existing fallback transaction timestamp was not recovered", err)
	}
	ticker, side, date := "HRTA", models.TypeSell, "2026-10-16"
	lot, price, fee, net := 28, 1615.0, 11363.0, 4510637.0
	if legacyBuy {
		ticker, side, date = "KREN", models.TypeBuy, "2025-07-28"
		lot, price, fee, net = 1, 9, 1, 901
	}
	if row.Ticker != ticker || row.Type != side || row.Price != price || row.Lot != lot || row.Tax != fee || row.AmountDone != net || row.Date.Format("2006-01-02") != date {
		t.Fatal("normalized trade differs from the source")
	}
	var source string
	if err := tx.Raw("SELECT consent_source FROM trade_reviews WHERE id = ?", review.ID).Scan(&source).Error; err != nil || source != "sync:docling:application/pdf" {
		t.Fatal("missing Docling provenance", err)
	}
	for i := 0; i < 2; i++ {
		processed, err := repository.Reviews.Get(t.Context(), user.Id, review.ID)
		if err != nil || processed.State != "saved" {
			t.Fatal("missing saved Docling provenance", err)
		}
		n, err := repository.Reviews.Save(t.Context(), user.Id, review.ID, "pending", []*models.Transaction{&row})
		if err != nil || n != 0 {
			t.Fatalf("repeat saved %d: %v", n, err)
		}
	}
	t.Logf("Saved and re-read %s %s, %d lots at %.0f, fee %.0f, net %.0f. Replays saved zero rows.", row.Type, row.Ticker, row.Lot, row.Price, row.Tax, row.AmountDone)
}

func TestDoclingAccountingAmount(t *testing.T) {
	for _, tc := range []struct {
		raw, want        string
		allowParentheses bool
	}{
		{" Rp (901.00) ", "901.00", true},
		{"(900.00)", "", false},
		{"4,522,000.00", "4522000.00", false},
		{"(4,52.00)", "", true},
	} {
		got, err := doclingAmount(tc.raw, tc.allowParentheses)
		if got != tc.want || (err == nil) != (tc.want != "") {
			t.Errorf("amount %q: got %q, err=%v", tc.raw, got, err)
		}
	}
}
