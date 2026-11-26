package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/service/parser"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type retryTradeParser struct{ recovered bool }

func (*retryTradeParser) BrokerName() string { return "Stockbit Sekuritas" }
func (p *retryTradeParser) Parse(data []byte) ([]dto.ParsedTrade, error) {
	if string(data) == "unknown" && !p.recovered {
		return nil, parser.ErrNoStockbitTradeRows
	}
	return []dto.ParsedTrade{{BrokerName: p.BrokerName(), Ticker: "MIDI", Type: "BUY", Lot: 1, Price: 284, RefNo: string(data)}}, nil
}

type retryTradeRepo struct {
	contract.TransactionRepository
	refs map[string]bool
}

func (r *retryTradeRepo) SyncTradeConfirmationBatch(_ uint, rows []*models.Transaction) (int, error) {
	n := 0
	for _, v := range rows {
		key := fmt.Sprintf("%d:%s", v.BrokerID, v.RefNo)
		if !r.refs[key] {
			r.refs[key] = true
			n++
		}
	}
	return n, nil
}

type retryTradeCache struct {
	contract.GmailCacheRepository
	cached map[string]bool
}

func (r *retryTradeCache) GetProcessedMessageIDs(context.Context, uint, string) (map[string]bool, error) {
	return r.cached, nil
}
func (r *retryTradeCache) MarkProcessed(_ context.Context, _ uint, id, _ string, _ int) error {
	r.cached[id] = true
	return nil
}

type retryTradeAuth struct{ contract.AuthRepository }

func (retryTradeAuth) UpdateUser(user *models.Users) (*models.Users, error) { return user, nil }

func TestUnknownTradePDFKeepsOtherTradesAndRetries(t *testing.T) {
	messages := map[string][]string{"mixed": {"unknown", "sibling"}, "later": {"valid"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/messages") {
			json.NewEncoder(w).Encode(&gmail.ListMessagesResponse{Messages: []*gmail.Message{{Id: "mixed"}, {Id: "later"}}})
			return
		}
		id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		payload := &gmail.MessagePart{}
		for _, ref := range messages[id] {
			payload.Parts = append(payload.Parts, &gmail.MessagePart{Filename: ref + ".pdf", MimeType: "application/pdf", Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString([]byte(ref))}})
		}
		json.NewEncoder(w).Encode(&gmail.Message{Id: id, InternalDate: 1760000000000, Payload: payload})
	}))
	defer server.Close()
	client, err := gmail.NewService(t.Context(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	p := &retryTradeParser{}
	repo := &retryTradeRepo{refs: map[string]bool{}}
	cache := &retryTradeCache{cached: map[string]bool{}}
	s := GmailServiceImpl{TradeParser: p, TransactionRepo: repo, GmailCacheRepo: cache, AuthRepository: retryTradeAuth{}}
	user := &models.Users{Id: 1}
	run := func() *dto.GmailSyncResponse {
		t.Helper()
		res, err := s.syncTradeConfirmationsWithDate(t.Context(), client, user, time.Now(), nil, "")
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := run()
	if res.TradesSynced != 2 || len(res.PendingPDFs) != 1 || res.PendingPDFs[0].Filename != "unknown.pdf" || user.LastGmailSyncAt != nil || cache.cached["mixed"] || !cache.cached["later"] {
		t.Fatalf("incomplete import: %+v, cache %v", res, cache.cached)
	}
	if res = run(); res.TradesSynced != 0 || len(res.PendingPDFs) != 1 {
		t.Fatalf("retry: %+v", res)
	}
	p.recovered = true
	if res = run(); res.TradesSynced != 1 || len(res.PendingPDFs) != 0 || user.LastGmailSyncAt == nil || !cache.cached["mixed"] || len(repo.refs) != 3 {
		t.Fatalf("repaired import: %+v", res)
	}
}

type brokerScopedRetryParser struct{ retryTradeParser }

func (p *brokerScopedRetryParser) EmailSelectors(context.Context) ([]parser.EmailMetadata, error) {
	return []parser.EmailMetadata{{From: "no-reply@stockbit.com", Subject: "Trade Confirmation"}}, nil
}

type reconnectBrokerRepo struct {
	contract.BrokerRepository
	broker models.Broker
}

func (r *reconnectBrokerRepo) GetAllBrokers(_ uint) ([]models.Broker, error) {
	return []models.Broker{r.broker}, nil
}

type scopedRetryCache struct {
	contract.GmailCacheRepository
	messages map[string]map[string]bool
}

func (c *scopedRetryCache) GetProcessedMessageIDs(_ context.Context, _ uint, kind string) (map[string]bool, error) {
	return c.messages[kind], nil
}

func (c *scopedRetryCache) MarkProcessed(_ context.Context, _ uint, id, kind string, _ int) error {
	if c.messages[kind] == nil {
		c.messages[kind] = make(map[string]bool)
	}
	c.messages[kind][id] = true
	return nil
}

func TestTradeSyncImportsHistoryForReconnectedBroker(t *testing.T) {
	const messageID = "history"
	fetched := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/messages") {
			json.NewEncoder(w).Encode(gmail.ListMessagesResponse{Messages: []*gmail.Message{{Id: messageID}}})
			return
		}
		fetched++
		attachment := &gmail.MessagePart{Filename: "tc.pdf", MimeType: "application/pdf", Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString([]byte("fill"))}}
		json.NewEncoder(w).Encode(gmail.Message{Id: messageID, InternalDate: 1760000000000, Payload: attachment})
	}))
	defer server.Close()
	client, err := gmail.NewService(t.Context(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	brokers := &reconnectBrokerRepo{broker: models.Broker{ID: 29, Name: "Stockbit Sekuritas"}}
	cache := &scopedRetryCache{messages: map[string]map[string]bool{"trade_confirmation:v3": {messageID: true}}}
	trades := &retryTradeRepo{refs: map[string]bool{}}
	s := GmailServiceImpl{TradeParser: &brokerScopedRetryParser{}, BrokerRepo: brokers, TransactionRepo: trades, GmailCacheRepo: cache}
	for _, step := range []struct{ id, saved, fetched int }{{29, 1, 1}, {29, 0, 1}, {30, 1, 2}, {30, 0, 2}} {
		brokers.broker.ID = step.id
		result, err := s.syncTradeConfirmationsWithDate(t.Context(), client, &models.Users{Id: 11}, time.Now(), nil, brokers.broker.Name)
		if err != nil || result.TradesSynced != step.saved || fetched != step.fetched {
			t.Fatalf("broker %d: result=%+v fetched=%d err=%v", step.id, result, fetched, err)
		}
	}
	if !trades.refs["30:fill"] {
		t.Fatal("history was not saved to new broker")
	}
}
