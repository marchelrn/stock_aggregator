package routes

import (
	"bytes"
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
)

type mockStocksService struct {
	lastUserID uint
	contract.StocksService
}

func (m *mockStocksService) GetAllStocks(userID uint, limiter int) (*dto.StocksResponse, error) {
	m.lastUserID = userID
	return &dto.StocksResponse{
		StatusCode: http.StatusOK,
		Message:    "Success",
		StocksData: []dto.StocksData{},
	}, nil
}

type mockBrokersService struct {
	lastUserID uint
	contract.BrokersService
}

func (m *mockBrokersService) GetAllBrokers(userID uint) (*dto.BasicBrokersResponse, error) {
	m.lastUserID = userID
	return &dto.BasicBrokersResponse{
		StatusCode: http.StatusOK,
		Message:    "Success",
		Data:       []dto.Broker{},
	}, nil
}

type mockTransactionService struct {
	lastUserID uint
	contract.TransactionService
}

func (m *mockTransactionService) GetAllTransactions(userID uint) (*dto.AllTransactionsResponse, error) {
	m.lastUserID = userID
	return &dto.AllTransactionsResponse{
		StatusCode: http.StatusOK,
		Message:    "Success",
		Data:       []dto.TransactionData{},
	}, nil
}

func (m *mockTransactionService) CreateTransaction(userID uint, ticker []string, payload *dto.TransactionRequest) (*dto.TransactionResponse, error) {
	m.lastUserID = userID
	return &dto.TransactionResponse{
		StatusCode: http.StatusOK,
		Message:    "Success",
	}, nil
}

func makeToken(t *testing.T, userID uint) string {
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   "test@example.com",
		"name":    "Test User",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("user-isolation-test-secret"))
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}
	return token
}

func TestEssentialRoutesRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ENV", "test")
	t.Setenv("JWT_SECRET", "user-isolation-test-secret")
	config.Load()

	stocksStub := &mockStocksService{}
	brokersStub := &mockBrokersService{}
	txStub := &mockTransactionService{}

	router := SetupRoutes(&contract.Service{
		Stocks:      stocksStub,
		Brokers:     brokersStub,
		Transaction: txStub,
	})

	protectedPaths := []struct {
		method string
		path   string
	}{
		{"GET", "/my/stocks"},
		{"GET", "/brokers"},
		{"GET", "/transactions"},
		{"POST", "/transaction"},
		{"POST", "/broker"},
		{"POST", "/my/stock"},
	}

	for _, tc := range protectedPaths {
		t.Run(tc.method+" "+tc.path+" unauthenticated", func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 Unauthorized for unauthenticated %s %s, got %d", tc.method, tc.path, rec.Code)
			}
		})
	}
}

func TestEssentialRoutesUseAuthenticatedUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ENV", "test")
	t.Setenv("JWT_SECRET", "user-isolation-test-secret")
	config.Load()

	stocksStub := &mockStocksService{}
	brokersStub := &mockBrokersService{}
	txStub := &mockTransactionService{}

	router := SetupRoutes(&contract.Service{
		Stocks:      stocksStub,
		Brokers:     brokersStub,
		Transaction: txStub,
	})

	// Test with User 1
	tokenUser1 := makeToken(t, 101)

	req1 := httptest.NewRequest("GET", "/my/stocks", nil)
	req1.Header.Set("Authorization", "Bearer "+tokenUser1)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec1.Code)
	}
	if stocksStub.lastUserID != 101 {
		t.Fatalf("expected stocks service to receive userID 101, got %d", stocksStub.lastUserID)
	}

	req2 := httptest.NewRequest("GET", "/brokers", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenUser1)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	if brokersStub.lastUserID != 101 {
		t.Fatalf("expected brokers service to receive userID 101, got %d", brokersStub.lastUserID)
	}

	req3 := httptest.NewRequest("GET", "/transactions", nil)
	req3.Header.Set("Authorization", "Bearer "+tokenUser1)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec3.Code)
	}
	if txStub.lastUserID != 101 {
		t.Fatalf("expected transaction service to receive userID 101, got %d", txStub.lastUserID)
	}

	// Test with User 2 (different user)
	tokenUser2 := makeToken(t, 202)

	req4 := httptest.NewRequest("GET", "/my/stocks", nil)
	req4.Header.Set("Authorization", "Bearer "+tokenUser2)
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec4.Code)
	}
	if stocksStub.lastUserID != 202 {
		t.Fatalf("expected stocks service to receive userID 202, got %d", stocksStub.lastUserID)
	}

	// Test transaction creation with User 2
	txPayload := dto.TransactionRequest{
		Ticker:     "BBCA",
		Price:      10000,
		Lot:        1,
		BrokerID:   1,
		BrokerName: "Stockbit Sekuritas",
		Type:       "BUY",
	}
	txBody, _ := json.Marshal(txPayload)
	req5 := httptest.NewRequest("POST", "/transaction", bytes.NewBuffer(txBody))
	req5.Header.Set("Authorization", "Bearer "+tokenUser2)
	req5.Header.Set("Content-Type", "application/json")
	rec5 := httptest.NewRecorder()
	router.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body: %s", rec5.Code, rec5.Body.String())
	}
	if txStub.lastUserID != 202 {
		t.Fatalf("expected transaction service to receive userID 202, got %d", txStub.lastUserID)
	}
}
