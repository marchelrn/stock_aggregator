package service

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/migrations"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/repository"
	"github.com/marchelrn/stock_api/service/parser"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestGmailCashHistoryAndCheckpoint(t *testing.T) {
	dsn := os.Getenv("GMAIL_HISTORY_TEST_DSN")
	if dsn == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	config.Load()
	migrations.Up(sqlDB)
	tx := db.Begin()
	defer tx.Rollback()
	r := repository.New(tx)
	user := &models.Users{Email: "history@example.test", CreatedAt: time.Now().Add(-24 * time.Hour)}
	if err := tx.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	broker, err := r.Brokers.AddBrokerData(&models.Broker{UserID: user.Id, Name: "Stockbit Sekuritas", Cash: 50})
	if err != nil {
		t.Fatal(err)
	}
	s := GmailServiceImpl{CashMutationRepo: r.CashMutations, BrokerRepo: r.Brokers, GmailCacheRepo: r.GmailCache,
		TradeParser: parser.NewBrokerDispatcherParser(parser.NewMetadataParser(r.PDFTemplates), nil, nil, nil)}

	mail := func(subject, text string, at time.Time) *gmail.Message {
		return &gmail.Message{InternalDate: at.UnixMilli(), Payload: &gmail.MessagePart{MimeType: "text/plain",
			Headers: []*gmail.MessagePartHeader{{Name: "From", Value: "no-reply@stockbit.com"}, {Name: "Subject", Value: subject}},
			Body:    &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString([]byte(text))}}}
	}
	old := user.CreatedAt.Add(-time.Hour)
	rows := map[string]*gmail.Message{
		"dep":  mail("Deposit Kamu Sudah Masuk", "Deposit sebesar Rp 100", old),
		"wd":   mail("Penarikan Dana Berhasil", "Jumlah : IDR 50 Nomor Referensi : WD01", old),
		"div":  mail("Pembagian dividen tunai BBRI", "Dividen Bersih Rp 25", old),
		"same": mail("Deposit Kamu Sudah Masuk", "Deposit sebesar Rp 100", old.Add(time.Second)),
	}
	rows["foreign-div"] = mail("Pembagian dividen tunai AAPL", "Dividen Bersih 25 USD", old)
	rows["foreign-wd"] = mail("Penarikan Dana Berhasil", "Jumlah : 50 USD Nomor Referensi : US01", old)
	rows["crypto-dep"] = mail("Deposit Kamu Sudah Masuk", "Deposit sebesar Rp 100. Produk: kripto", old)
	order := []string{"dep", "foreign-wd", "wd", "div", "foreign-div", "crypto-dep", "same"}
	failSearch := false
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := path.Base(req.URL.Path)
		if id == "messages" {
			queries = append(queries, req.URL.Query().Get("q"))
			if failSearch {
				http.Error(w, "{}", http.StatusForbidden)
				return
			}
			var list gmail.ListMessagesResponse
			for _, id := range order {
				list.Messages = append(list.Messages, &gmail.Message{Id: id})
			}
			json.NewEncoder(w).Encode(list)
			return
		}
		msg := *rows[id]
		msg.Id = id
		json.NewEncoder(w).Encode(msg)
	}))
	defer server.Close()
	client, err := gmail.NewService(t.Context(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	check := func(selected string, wantRows int, balance float64, incremental bool) {
		t.Helper()
		result, err := s.syncBrokerCash(t.Context(), client, user, selected)
		if err != nil || result.MutationsSynced != wantRows {
			t.Fatalf("sync: %+v %v", result, err)
		}
		q := queries[len(queries)-1]
		if strings.Contains(q, "after:") != incremental || !strings.Contains(q, "stockbit.com") || strings.Contains(q, "trimegah") {
			t.Fatalf("query %q", q)
		}
		stored, err := r.Brokers.GetBrokerByName(user.Id, broker.Name)
		if err != nil || stored.Cash != balance || stored.LastCashSyncAt == nil {
			t.Fatalf("broker %+v %v", stored, err)
		}
	}
	check(broker.Name, 4, 50, false)
	// Gmail returns the most recent notification first.
	now := time.Now().Add(time.Minute)
	rows["live-wd"] = mail("Penarikan Dana Berhasil", "Jumlah : IDR 125 Nomor Referensi : WD02", now.Add(time.Second))
	rows["live-dep"] = mail("Deposit Kamu Sudah Masuk", "Deposit sebesar Rp 100", now)
	order = []string{"live-wd", "live-dep", "dep"}
	check("", 2, 25, true)
	check(broker.Name, 0, 25, false)
	before, _ := r.Brokers.GetBrokerByName(user.Id, broker.Name)
	failSearch = true
	if _, err := s.syncBrokerCash(t.Context(), client, user, ""); err == nil {
		t.Fatal("search failure hidden")
	}
	after, _ := r.Brokers.GetBrokerByName(user.Id, broker.Name)
	if !before.LastCashSyncAt.Equal(*after.LastCashSyncAt) {
		t.Fatal("failed search advanced checkpoint")
	}
}

func TestGmailCacheRequiresEveryStoredRow(t *testing.T) {
	dsn := os.Getenv("GMAIL_HISTORY_TEST_DSN")
	if dsn == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	config.Load()
	migrations.Up(sqlDB)
	tx := db.Begin()
	defer tx.Rollback()
	r := repository.New(tx)
	user := models.Users{Email: "gmail-verified-cache@example.test", CreatedAt: time.Now()}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	broker, err := r.Brokers.AddBrokerData(&models.Broker{UserID: user.Id, Name: "Ajaib Sekuritas"})
	if err != nil {
		t.Fatal(err)
	}
	batch := []*models.Transaction{
		{BrokerID: broker.ID, BrokerName: broker.Name, Ticker: "HRTA", Type: "BUY", Date: time.Now().Add(-time.Hour), Lot: 1, Price: 1600, RefNo: "test1"},
		{BrokerID: broker.ID, BrokerName: broker.Name, Ticker: "HRTA", Type: "BUY", Date: time.Now().Add(-time.Hour), Lot: 2, Price: 1600, RefNo: "test2"},
	}
	review, err := r.Reviews.Remember(t.Context(), &models.TradeReview{UserID: user.Id, BrokerID: broker.ID, BrokerName: broker.Name, MessageID: "pdf", Digest: "verify", State: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	if count, err := r.Reviews.Save(t.Context(), user.Id, review.ID, "pending", batch); err != nil || count != 2 {
		t.Fatalf("save=%d err=%v", count, err)
	}
	cache := repository.ImplGmailCacheRepository(tx)
	scope := gmailCacheScope(tradeConfirmationCacheType, map[string]int{broker.Name: broker.ID})
	if err := cache.MarkVerified(t.Context(), user.Id, "pdf", scope, 2, []int{batch[0].Id, batch[1].Id}); err != nil {
		t.Fatal(err)
	}
	if err := cache.MarkProcessed(t.Context(), user.Id, "legacy", scope, 2); err != nil {
		t.Fatal(err)
	}
	assertCache := func(want bool) {
		ids, err := cache.GetProcessedMessageIDs(t.Context(), user.Id, scope)
		if err != nil || ids["pdf"] != want || ids["legacy"] {
			t.Fatalf("eligible cache=%v, err=%v", ids, err)
		}
	}
	assertCache(true)
	if err := r.Transactions.DeleteTransaction(user.Id, batch[1].Id); err != nil {
		t.Fatal(err)
	}
	assertCache(false)
	pending, err := r.Reviews.List(t.Context(), user.Id, false)
	if err != nil || len(pending) != 1 || pending[0].State != "pending" {
		t.Fatalf("review did not reopen: %v", err)
	}
	if count, err := r.Reviews.Save(t.Context(), user.Id, review.ID, "pending", batch); err != nil || count != 1 {
		t.Fatalf("restored=%d err=%v", count, err)
	}
	if err := cache.MarkVerified(t.Context(), user.Id, "pdf", scope, 2, []int{batch[0].Id, batch[1].Id}); err != nil {
		t.Fatal(err)
	}
	assertCache(true)
}
