package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"github.com/marchelrn/stock_api/service"
)

type marketStub struct {
	contract.YahooService
	calls int
	err   error
}

func (s *marketStub) GetMarket(ctx context.Context, ticker string) (*dto.MarketResponse, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return &dto.MarketResponse{Ticker: ticker, FetchedAt: "2026-10-08T00:00:00Z", History: []dto.MarketPoint{}}, nil
}

func TestMarketProtectedRouteAndEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ENV", "test")
	t.Setenv("JWT_SECRET", "market-route-test-secret")
	config.Load()
	stub := &marketStub{}
	router := SetupRoutes(&contract.Service{Yahoo: stub})

	request := httptest.NewRequest(http.MethodGet, "/api/market/BBCA", nil)
	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, request)
	if unauthenticated.Code != http.StatusUnauthorized || stub.calls != 0 {
		t.Fatalf("route is not protected: status=%d calls=%d", unauthenticated.Code, stub.calls)
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": 1, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("market-route-test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/market/BBCA", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || stub.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.calls, recorder.Body.String())
	}
	var envelope struct {
		Data dto.MarketResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Ticker != "BBCA" || envelope.Data.History == nil || envelope.Data.Price != nil {
		t.Fatalf("unexpected envelope: %s", recorder.Body.String())
	}
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	keys := []string{"ticker", "name", "currency", "exchange", "price", "previous_close", "change", "change_percent", "day_high", "day_low", "fifty_two_week_high", "fifty_two_week_low", "volume", "market_time", "fetched_at", "history"}
	if len(raw["data"]) != len(keys) {
		t.Fatalf("unexpected contract fields: %s", recorder.Body.String())
	}
	for _, key := range keys {
		if _, ok := raw["data"][key]; !ok {
			t.Errorf("missing field %q", key)
		}
	}

	stub.err = errs.TooManyRequests("Yahoo chart returned HTTP 429")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request.Clone(context.Background()))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("handler concealed upstream 429: %d %s", recorder.Code, recorder.Body.String())
	}
	var failure struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &failure); err != nil || failure.StatusCode != 429 || failure.Message == "" {
		t.Fatalf("unexpected error envelope: %s", recorder.Body.String())
	}

	// Use the real service for validation without making any upstream request.
	router = SetupRoutes(&contract.Service{Yahoo: service.ImplYahooService(nil)})
	request = httptest.NewRequest(http.MethodGet, "/api/market/BBCA.JK", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid ticker status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
