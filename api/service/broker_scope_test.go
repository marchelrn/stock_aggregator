package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/service/parser"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type scopedTemplates []models.PDFParsingTemplate

func (s scopedTemplates) ListActive(context.Context) ([]models.PDFParsingTemplate, error) {
	return s, nil
}

type scopedBrokers struct{ contract.BrokerRepository }

func (scopedBrokers) GetAllBrokers(uint) ([]models.Broker, error) {
	return []models.Broker{{ID: 1, Name: "Stockbit Sekuritas"}, {ID: 2, Name: "Trimegah Sekuritas"}}, nil
}
func TestSetupSearchesOnlySelectedBroker(t *testing.T) {
	var store scopedTemplates
	for _, name := range []string{"trades_v2", "cash_v2"} {
		data, err := os.ReadFile("../migrations/templates/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var entries scopedTemplates
		if err = json.Unmarshal(data, &entries); err != nil {
			t.Fatal(err)
		}
		for i := range entries {
			entries[i].Active = true
		}
		store = append(store, entries...)
	}
	svc := GmailServiceImpl{BrokerRepo: scopedBrokers{}, TradeParser: parser.NewBrokerDispatcherParser(parser.NewMetadataParser(store), parser.NewStockbitParser(), parser.NewMandiriParser(), parser.NewTrimegahParser())}
	user := &models.Users{Id: 1, CreatedAt: time.Now()}
	for _, tc := range []struct{ broker, want, exclude string }{{"Stockbit Sekuritas", "stockbit", "trimegah"}, {"Trimegah Sekuritas", "trimegah", "stockbit"}} {
		t.Run(tc.broker, func(t *testing.T) {
			queries := make(chan string, 2)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				queries <- r.URL.Query().Get("q")
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("{}"))
			}))
			defer srv.Close()
			client, err := gmail.NewService(t.Context(), option.WithEndpoint(srv.URL+"/"), option.WithoutAuthentication())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = svc.syncTradeConfirmationsWithDate(t.Context(), client, user, time.Now(), nil, tc.broker); err != nil {
				t.Fatal(err)
			}
			if _, err = svc.syncBrokerCash(t.Context(), client, user, tc.broker); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				q := strings.ToLower(<-queries)
				if !strings.Contains(q, tc.want) || strings.Contains(q, tc.exclude) {
					t.Fatalf("incorrect broker search: %s", q)
				}
			}
			if user.LastGmailSyncAt != nil {
				t.Fatal("manual sync moved account checkpoint")
			}
		})
	}
	if _, _, err := svc.brokerScope(t.Context(), 1, "Other"); err == nil {
		t.Fatal("unregistered broker accepted")
	}
}
