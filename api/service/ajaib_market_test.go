package service

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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
	"gorm.io/gorm/logger"
)

func TestAjaibMarketFilterRealAttachments(t *testing.T) {
	dir, local := os.Getenv("AJAIB_EXCLUDED_SAMPLE_DIR"), os.Getenv("AJAIB_SAMPLE_PDF")
	if dir == "" || local == "" {
		t.Skip("requires local IDX and excluded Ajaib samples")
	}
	raw, err := os.ReadFile("../migrations/templates/ajaib_trade.json")
	if err != nil {
		t.Fatal(err)
	}
	var seed models.PDFParsingTemplate
	if err := json.Unmarshal(raw, &seed); err != nil {
		t.Fatal(err)
	}
	seed.Active = true
	metadata := parser.NewMetadataParser(ajaibTestTemplates{seed})
	svc := &GmailServiceImpl{TradeParser: parser.NewBrokerDispatcherParser(metadata, parser.NewStockbitParser(), parser.NewMandiriParser(), parser.NewTrimegahParser())}
	ctx := parser.WithBrokerScope(t.Context(), map[string]int{"ajaib sekuritas": 1})
	paths, err := filepath.Glob(filepath.Join(dir, "*.pdf"))
	if err != nil || len(paths) == 0 {
		t.Fatal("exclude sample directory is empty", err)
	}
	paths = append(paths, local)
	for i, path := range paths {
		pdf, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		msg := &gmail.Message{Id: "sample", Payload: &gmail.MessagePart{
			Filename: "TC_same_selector.pdf", MimeType: "application/pdf",
			Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString(pdf)},
			Headers: []*gmail.MessagePartHeader{
				{Name: "From", Value: seed.EmailSender},
				{Name: "Subject", Value: seed.SubjectContains},
			},
		}}
		skipped, recovered := 0, 0
		trades, pending, err := svc.processSyncAttachments(ctx, nil, msg, 1, nil, nil, &skipped, &recovered, new(int))
		if err != nil || len(pending) != 0 || recovered != 0 {
			t.Fatalf("sample %d: unexpected processing outcome", i)
		}
		if path == local {
			if len(trades) != 1 || skipped != 0 || trades[0].Ticker != "HRTA" || trades[0].Lot != 28 {
				t.Fatal("IDX sample did not reach normal parser")
			}
		} else if skipped != 1 || len(trades) != 0 {
			t.Fatalf("excluded sample %d reached parsing", i)
		}
	}
	t.Logf("Excluded %d non-IDX documents and parsed one IDX confirmation.", len(paths)-1)
}

func TestAjaibHistoricalDomainDiscovery(t *testing.T) {
	dsn, sample := os.Getenv("GMAIL_HISTORY_TEST_DSN"), os.Getenv("AJAIB_SAMPLE_PDF")
	ticker, tradeDate, tradeType := "HRTA", "2026-10-16", models.TypeSell
	lot, price, net := 28, 1615.0, 4510637.0
	subject := "Historical account trade"
	if legacySample := os.Getenv("AJAIB_LEGACY_SAMPLE_PDF"); legacySample != "" {
		sample = legacySample
		ticker, tradeDate, tradeType = "KREN", "2025-07-28", models.TypeBuy
		lot, price, net = 1, 9, 901
		subject = "Rangkuman Transaksi Saham Harian (28 July 2025)"
	}

	secondPage := false
	if multiPage := os.Getenv("AJAIB_MULTIPAGE_SAMPLE_PDF"); multiPage != "" {
		sample, secondPage = multiPage, true
		ticker, tradeDate, tradeType = "SIDO", "2024-09-12", models.TypeBuy
		lot, price, net = 1, 680, 68102
	}
	expectedCount := 1
	if secondPage {
		expectedCount++
	}

	if dsn == "" || sample == "" {
		t.Skip("requires disposable PostgreSQL and an Ajaib IDX PDF")
	}
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
	tx := db.Begin()
	defer tx.Rollback()
	store := repository.New(tx)
	services, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	svc := services.Gmail.(*GmailServiceImpl)
	user := models.Users{Email: "ajaib-domain@example.test", CreatedAt: time.Now()}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	broker, err := store.Brokers.AddBrokerData(&models.Broker{UserID: user.Id, Name: ajaibBroker})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := os.ReadFile(sample)
	if err != nil {
		t.Fatal(err)
	}

	foreign := exec.CommandContext(t.Context(), "ps2pdf", "-", "-")
	foreign.Stdin = strings.NewReader("%!PS\n/Courier findfont 12 scalefont setfont\n30 730 moveto (Trade Confirmation - Saham AS) show\n30 700 moveto (PT Ajaib Futures) show\nshowpage\n")
	usPDF, err := foreign.Output()
	if err != nil {
		t.Fatal(err)
	}

	ctx := parser.WithBrokerScope(t.Context(), map[string]int{"ajaib sekuritas": broker.ID})
	dispatcher := svc.TradeParser.(*parser.BrokerDispatcherParser)
	foreignDomain := parser.EmailMetadata{From: "actual@ajaib.co.id.example.test", Subject: "Historical account trade", Filename: "TC_confirmation.pdf"}
	if name, err := dispatcher.ReviewBroker(ctx, foreignDomain); err != nil || name != "" {
		t.Fatalf("lookalike domain selected broker %q: %v", name, err)
	}

	for _, name := range []string{"TC_20240912_O0CP1B.pdf", "tc-legacy.PDF", "soa_20240912.pdf", "monthly-report.pdf", "notice_TC.pdf"} {
		brokerName, excluded, err := dispatcher.ReviewAttachment(ctx, parser.EmailMetadata{From: "statements@ajaib.co.id", Filename: name})
		isTC := strings.HasPrefix(strings.ToLower(name), "tc_") || strings.HasPrefix(strings.ToLower(name), "tc-")
		if err != nil || excluded == isTC || (brokerName == ajaibBroker) != isTC {
			t.Fatalf("route %q: broker=%q excluded=%t err=%v", name, brokerName, excluded, err)
		}
	}

	sentAt := time.Date(2026, 10, 18, 21, 14, 15, 0, time.FixedZone("WITA", 8*3600))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/messages") {
			q := r.URL.Query().Get("q")
			if !strings.Contains(q, `from:"ajaib.co.id"`) || strings.Contains(q, "subject:") || strings.Contains(q, "after:") || !strings.Contains(q, "filename:pdf") {
				t.Errorf("unexpected historical query %q", q)
			}
			json.NewEncoder(w).Encode(gmail.ListMessagesResponse{Messages: []*gmail.Message{{Id: "domestic"}, {Id: "us"}, {Id: "non_tc"}}})
			return
		}
		id := filepath.Base(r.URL.Path)
		source := pdf
		filename := "TC_confirmation.pdf"
		if id == "us" {
			source = usPDF
		}
		if id == "non_tc" {
			json.NewEncoder(w).Encode(gmail.Message{Id: id, Payload: &gmail.MessagePart{
				Filename: "SOA_20240912.pdf", MimeType: "application/pdf",
				Headers: []*gmail.MessagePartHeader{{Name: "From", Value: "statements@ajaib.co.id"}},
			}})
			return
		}
		json.NewEncoder(w).Encode(gmail.Message{Id: id, InternalDate: time.Now().UnixMilli(), Payload: &gmail.MessagePart{
			Filename: filename, MimeType: "application/pdf",
			Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString(source)},
			Headers: []*gmail.MessagePartHeader{
				{Name: "From", Value: "Ajaib Statements <statements@ajaib.co.id>"},
				{Name: "Subject", Value: subject},
				{Name: "Date", Value: sentAt.Format(time.RFC1123Z)},
			},
		}})
	}))
	defer server.Close()
	client, err := gmail.NewService(t.Context(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.syncTradeConfirmationsWithDate(t.Context(), client, &user, time.Now(), nil, broker.Name)
	if err != nil || result.TradesSynced != expectedCount || result.EmailsFound != 3 || result.PDFsExcluded != 2 || len(result.PendingPDFs) != 0 {
		t.Fatalf("historical domain sync: %+v, %v", result, err)
	}
	var rows []models.Transaction
	if err := tx.Where("user_id=? AND broker_id=?", user.Id, broker.ID).Order("id").Find(&rows).Error; err != nil || len(rows) != expectedCount || rows[0].Ticker != ticker || rows[0].Date.Format("2006-01-02") != tradeDate || rows[0].Type != tradeType || rows[0].Lot != lot || rows[0].Price != price || rows[0].AmountDone != net {
		t.Fatal("domestic TC not stored correctly", err)
	}
	for _, row := range rows {
		if row.EmailSentAt == nil || !row.EmailSentAt.Equal(sentAt) {
			t.Fatal("Gmail Date header not stored independently of trade date")
		}
	}
	if secondPage {
		sale := rows[1]
		if sale.Ticker != ticker || sale.Date.Format("2006-01-02") != tradeDate || sale.Type != models.TypeSell ||
			sale.Lot != 1 || sale.Price != 680 || sale.AmountDone != 67641 || sale.Tax != 359 {
			t.Fatal("sell confirmation was not independently reconciled")
		}
		if rows[0].Tax != 102 || rows[0].RefNo == sale.RefNo {
			t.Fatal("buy fee or distinct references were not preserved")
		}
	}

}
