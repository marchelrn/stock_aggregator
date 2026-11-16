package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"google.golang.org/api/gmail/v1"
)

type cashBrokerSpy struct {
	contract.BrokerRepository
	userID   uint
	id       int
	cash     float64
	asOf     time.Time
	from, to int
	amount   float64
}

func (r *cashBrokerSpy) SetCashBase(userID uint, id int, cash float64, asOf time.Time) (*models.Broker, error) {
	r.userID, r.id, r.cash, r.asOf = userID, id, cash, asOf
	return &models.Broker{ID: id, Cash: cash, CashBase: cash, CashBaseAsOf: &asOf}, nil
}
func (r *cashBrokerSpy) GetCashBreakdown(userID uint, id int) (*dto.CashBreakdown, error) {
	return &dto.CashBreakdown{BrokerId: id, Cash: r.cash, CashBase: r.cash, CashBaseAsOf: &r.asOf, Events: []dto.CashEvent{}}, nil
}
func (r *cashBrokerSpy) UpdateBrokerData(userID uint, id int, broker *models.Broker) (*models.Broker, error) {
	r.userID, r.id, r.cash = userID, id, broker.Cash
	return broker, nil
}
func (r *cashBrokerSpy) TransferCash(userID uint, from, to int, amount float64) error {
	r.userID, r.from, r.to, r.amount = userID, from, to, amount
	return nil
}

func TestCashSetCurrentBalanceContract(t *testing.T) {
	for _, body := range []string{
		`{"cash_base":1234}`,
		`{"cash_base":1234,"cash_base_as_of":"2000-01-01T00:00:00Z"}`,
		`{"cash_base":1234,"cash_base_as_of":"2100-01-01T00:00:00Z"}`,
	} {
		t.Run(body, func(t *testing.T) {
			var request dto.SetCashBaseRequest
			if err := json.Unmarshal([]byte(body), &request); err != nil {
				t.Fatal(err)
			}
			repo := &cashBrokerSpy{}
			svc := ImplBrokersService(repo)
			start := time.Now()
			response, err := svc.SetCashBase(7, 9, &request)
			if err != nil {
				t.Fatal(err)
			}
			if repo.userID != 7 || repo.id != 9 || repo.cash != 1234 || repo.asOf.Before(start) || repo.asOf.After(time.Now()) {
				t.Fatalf("reset must ignore client time and use current balance: %+v", repo)
			}
			if response.Data.Cash != 1234 || len(response.Data.Events) != 0 {
				t.Fatalf("unexpected response: %+v", response)
			}
		})
	}
}

func TestCashAddAndTransferUseAtomicRepositoryOperations(t *testing.T) {
	repo := &cashBrokerSpy{}
	svc := ImplBrokersService(repo)
	if _, err := svc.UpdateBrokerData(7, 9, &dto.BrokerRequest{Cash: 250}); err != nil {
		t.Fatal(err)
	}
	if repo.userID != 7 || repo.id != 9 || repo.cash != 250 {
		t.Fatalf("addition must forward delta, not an absolute/stale balance: %+v", repo)
	}
	if _, err := svc.TransferCashBetweenBrokers(7, &dto.TransferCashRequest{FromBrokerId: 9, ToBrokerId: 10, Amount: 100}); err != nil {
		t.Fatal(err)
	}
	if repo.userID != 7 || repo.from != 9 || repo.to != 10 || repo.amount != 100 {
		t.Fatalf("transfer not delegated atomically: %+v", repo)
	}
}

type cashSyncRepoSpy struct {
	contract.CashMutationRepository
	items []*models.CashMutation
	err   error
}

func (r *cashSyncRepoSpy) SyncCashMutationBatch(_ uint, items []*models.CashMutation) (int, error) {
	r.items = items
	if r.err != nil {
		return 0, r.err
	}
	return len(items), nil
}

func TestGmailCashSyncRegistrationAndPersistenceFailure(t *testing.T) {
	for _, historical := range []bool{false, true} {
		t.Run(map[bool]string{false: "eligible persistence error", true: "historical skipped"}[historical], func(t *testing.T) {
			received := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
			registered := received.Add(-24 * time.Hour)
			if historical {
				registered = received.Add(24 * time.Hour)
			}
			client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/messages") {
					if !strings.Contains(r.URL.Query().Get("q"), fmt.Sprintf(" after:%d", registered.Unix())) {
						t.Error("cash query must be bounded by registration")
					}
					gmailJSON(t, w, &gmail.ListMessagesResponse{Messages: []*gmail.Message{{Id: "cash"}}})
					return
				}
				gmailJSON(t, w, &gmail.Message{Id: "cash", InternalDate: received.UnixMilli(), Payload: &gmail.MessagePart{
					MimeType: "text/html",
					Headers:  []*gmail.MessagePartHeader{{Name: "From", Value: "Stockbit <no-reply@stockbit.com>"}, {Name: "Subject", Value: "Deposit Kamu Sudah Masuk"}},
					Body:     &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString([]byte(`<div>deposit yang kamu lakukan sebesar <strong>Rp 353,500</strong> sudah masuk ke RDN</div>`))},
				}})
			})
			wantErr := errors.New("balance persistence failed")
			repo := &cashSyncRepoSpy{err: wantErr}
			svc := &GmailServiceImpl{CashMutationRepo: repo}
			checkpoint := received.Add(time.Hour)
			user := &models.Users{Id: 7, CreatedAt: registered, LastGmailSyncAt: &checkpoint}
			if historical {

				response, err := svc.syncCashMutations(context.Background(), client, user)
				if err != nil || response.MutationsSynced != 0 || response.MutationsFound != 0 || len(repo.items) != 0 {
					t.Fatalf("historical mutation must not be imported: response=%+v items=%+v err=%v", response, repo.items, err)
				}
				return
			}
			if response, err := svc.syncCashMutations(context.Background(), client, user); !errors.Is(err, wantErr) || response != nil {
				t.Fatalf("failed cash persistence must not report success: response=%+v err=%v", response, err)
			}
			if len(repo.items) != 1 || repo.items[0].Date.Format("2006-01-02") != "2026-01-01" || repo.items[0].EmailReceivedAt == nil {
				t.Fatalf("cash sync must preserve historical date and receipt metadata: %+v", repo.items)
			}
			if !user.LastGmailSyncAt.Equal(checkpoint) {
				t.Fatal("cash sync changed the trade checkpoint")
			}
		})
	}
}
