package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
		if !r.refs[v.RefNo] {
			r.refs[v.RefNo] = true
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
