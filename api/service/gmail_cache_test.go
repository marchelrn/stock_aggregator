package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/service/parser"
	"google.golang.org/api/gmail/v1"
)

type syncCacheSpy struct {
	contract.GmailCacheRepository
	ids         map[string]bool
	marked      map[string]int
	lookupTypes []string
}

func (c *syncCacheSpy) GetProcessedMessageIDs(_ context.Context, _ uint, kind string) (map[string]bool, error) {
	c.lookupTypes = append(c.lookupTypes, kind)
	return c.ids, nil
}

func (c *syncCacheSpy) MarkProcessed(_ context.Context, _ uint, id, kind string, count int) error {
	c.marked[id] = count
	c.ids[id] = true
	return nil
}

type cacheTransactionSpy struct {
	contract.TransactionRepository
	cache *syncCacheSpy
	err   error
	calls int
	t     *testing.T
}

func (r *cacheTransactionSpy) SyncTradeConfirmationBatch(_ uint, trades []*models.Transaction) (int, error) {
	r.calls++
	if len(r.cache.marked) != 0 {
		r.t.Error("messages cached before transactions persisted")
	}
	return len(trades), r.err
}

func TestGmailCacheOnlyAfterPersistence(t *testing.T) {
	for _, name := range []string{"success", "save failure", "parse only", "skipped PDF", "later message fails", "cached message"} {
		t.Run(name, func(t *testing.T) {
			cache := &syncCacheSpy{ids: map[string]bool{}, marked: map[string]int{}}
			user := &models.Users{Id: 7}
			auth := &gmailAuthRepo{user: user}
			tx := &cacheTransactionSpy{cache: cache, t: t}
			if name == "save failure" {
				tx.err = errors.New("database unavailable")
			}
			if name == "cached message" {
				cache.ids["m1"] = true
			}
			p := &gmailSelectorParser{selectors: []parser.EmailMetadata{{From: "trim.operation@trimegah.com", Subject: "TC"}}}
			p.parseEmail = func(meta parser.EmailMetadata) ([]dto.ParsedTrade, error) {
				if name == "skipped PDF" {
					return nil, parser.ErrEmailNotApplicable
				}
				return []dto.ParsedTrade{{Ticker: "BBCA", Lot: 1, Price: 100, BrokerName: "Trimegah Sekuritas"}}, nil
			}
			svc := &GmailServiceImpl{AuthRepository: auth, TransactionRepo: tx, TradeParser: p, GmailCacheRepo: cache}
			if name == "parse only" {
				svc.TransactionRepo = nil
			}
			gets := 0
			client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/messages") {
					messages := []*gmail.Message{{Id: "m1"}}
					if name == "later message fails" {
						messages = append(messages, &gmail.Message{Id: "m2"})
					}
					gmailJSON(t, w, &gmail.ListMessagesResponse{Messages: messages})
					return
				}
				gets++
				if strings.HasSuffix(r.URL.Path, "/m2") {
					gmailJSON(t, w, &gmail.Message{Id: "m2"})
					return
				}
				gmailJSON(t, w, &gmail.Message{Id: "m1", Payload: &gmail.MessagePart{
					Filename: "TC_test.pdf", Body: &gmail.MessagePartBody{Data: "UERG"},
					Headers: []*gmail.MessagePartHeader{{Name: "From", Value: "trim.operation@trimegah.com"}, {Name: "Subject", Value: "TC"}},
				}})
			})
			_, err := svc.syncTradeConfirmationsWithDate(context.Background(), client, user, time.Now(), nil)
			wantErr := name == "save failure" || name == "later message fails"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v, wantErr=%v", err, wantErr)
			}
			wantMarks := 0
			if name == "success" {
				wantMarks = 1
			}
			if len(cache.marked) != wantMarks {
				t.Fatalf("cached=%v", cache.marked)
			}
			if cache.lookupTypes[0] != "trade_confirmation:v2" {
				t.Fatal("legacy unsafe cache must not skip retry")
			}
			if name == "cached message" && (gets != 0 || tx.calls != 0) {
				t.Fatal("cached message downloaded or persisted again")
			}
			if wantErr && auth.updates != 0 {
				t.Fatal("failed sync advanced checkpoint")
			}
		})
	}
}
