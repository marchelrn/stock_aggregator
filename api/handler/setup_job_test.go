package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
)

type setupTestBrokers struct{ contract.BrokersService }

func (setupTestBrokers) GetAllBrokers(uint) (*dto.BasicBrokersResponse, error) {
	return &dto.BasicBrokersResponse{Data: []dto.Broker{{Name: "Broker A"}}}, nil
}

type setupTestGmail struct {
	contract.GmailService
	release    chan struct{}
	tradeCalls chan string
	cashErr    error
}

func (g *setupTestGmail) SyncTradeConfirmationsFullContext(ctx context.Context, _ uint, broker string) (*dto.GmailSyncResponse, error) {
	g.tradeCalls <- broker
	select {
	case <-g.release:
		return nil, errors.New("PDF layout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *setupTestGmail) SyncCashMutationsContext(ctx context.Context, _ uint, _ string) (*dto.GmailCashSyncResponse, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return &dto.GmailCashSyncResponse{MutationsSynced: 1}, g.cashErr
}

func TestSetupSurvivesDisconnectAndReportsStages(t *testing.T) {
	for _, bothFailed := range []bool{false, true} {
		g := &setupTestGmail{release: make(chan struct{}), tradeCalls: make(chan string, 2)}
		if bothFailed {
			g.cashErr = errors.New("Gmail unavailable")
		}
		c := &GmailController{}
		c.InitService(&contract.Service{Gmail: g, Brokers: setupTestBrokers{}})
		router := gin.New()
		router.Use(func(ctx *gin.Context) {
			id := float64(1)
			if ctx.GetHeader("X-Test-User") == "other" {
				id = 2
			}
			ctx.Set("user_id", id)
		})
		router.POST("/sync", c.SyncSetupHistorical)
		router.GET("/sync/:id", c.SetupJobStatus)
		do := func(method, path, body, user string, expected int) map[string]any {
			t.Helper()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			req := httptest.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-User", user)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != expected {
				t.Fatalf("HTTP %d, body %s", w.Code, w.Body)
			}
			var out map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			return out
		}
		const body = `{"broker_name":"Broker A"}`
		do(http.MethodPost, "/sync?async=1", `{"broker_name":"Other"}`, "", 400)
		id := do(http.MethodPost, "/sync?async=1", body, "", 202)["job_id"].(string)
		if <-g.tradeCalls != "Broker A" {
			t.Fatal("incorrect broker")
		}
		if do(http.MethodPost, "/sync?async=1", body, "", 202)["job_id"] != id {
			t.Fatal("duplicate worker")
		}
		path := "/sync/" + id
		do(http.MethodGet, path, "", "other", 404)
		do(http.MethodGet, path, "", "", 202)
		close(g.release)
		deadline := time.Now().Add(time.Second)
		for {
			c.jobsMu.Lock()
			done := c.jobs[id].result != nil
			c.jobsMu.Unlock()
			if done {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("worker did not finish")
			}
			time.Sleep(time.Millisecond)
		}
		result := do(http.MethodGet, path, "", "", 200)["result"].(map[string]any)
		if result["trade_error"] != "Transaksi saham: PDF layout" {
			t.Fatalf("trade error lost: %v", result)
		}
		if bothFailed {
			if result["cash_error"] != "Mutasi kas: Gmail unavailable" {
				t.Fatalf("cash error lost: %v", result)
			}
		} else if result["cash_sync"].(map[string]any)["mutations_synced"] != float64(1) {
			t.Fatalf("cash import lost: %v", result)
		}
	}
}
