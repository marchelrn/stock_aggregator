package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/service/parser"
)

type ajaibTestTemplates []models.PDFParsingTemplate

func (t ajaibTestTemplates) ListActive(context.Context) ([]models.PDFParsingTemplate, error) {
	return t, nil
}

func TestLiveAjaibVariant(t *testing.T) {
	path := os.Getenv("AJAIB_ANON_VARIANT")
	if path == "" {
		t.Skip("requires an anonymized Ajaib layout variant")
	}
	pdf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile("../migrations/templates/ajaib_trade.json")
	if err != nil {
		t.Fatal(err)
	}
	var seed models.PDFParsingTemplate
	if err = json.Unmarshal(template, &seed); err != nil {
		t.Fatal(err)
	}
	seed.Active = true
	if rows, err := parser.NewMetadataParser(ajaibTestTemplates{seed}).Parse(pdf); err == nil || len(rows) > 0 {
		t.Fatal("expected changed layout to miss local template")
	}
	if err := godotenv.Load(os.Getenv("AJAIB_TEST_ENV")); err != nil {
		t.Fatal("test configuration unavailable")
	}
	key, model, err := geminiConfig()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()
	trades, err := extractValidatedPDF(ctx, key, model, pdf)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 {
		t.Fatal("unexpected trade count")
	}
	r := trades[0]
	if r.Date.Format("2006-01-02") != "2026-10-16" || r.Type != "SELL" || r.Ticker != "HRTA" || r.Price != 1615 || r.Lot != 28 || r.AmountDone != 4510637 || r.Tax != 11363 {
		t.Fatal("validated trade differs from source")
	}
}

func TestNormalizeAjaibDraft(t *testing.T) {
	d := dto.TradeDraft{Date: "2025-03-25", Rows: []dto.ReviewFill{{Ticker: "HRTA", Side: "SELL", Shares: "100", Price: "100", Gross: "10000"}}, SellGross: "10000", SellNet: "10000"}
	d.Rows[0].Ticker = " " + strings.ToLower(d.Rows[0].Ticker) + " "
	d.Rows[0].Side = map[string]string{"BUY": " beli ", "SELL": " jual "}[d.Rows[0].Side]
	d.Rows[0].Lots = ""
	if _, err := validateTradeDraft(d); err != nil {
		t.Fatal(err)
	}
	d.Rows[0].Side = "RG"
	if _, err := validateTradeDraft(d); err == nil || !strings.Contains(err.Error(), "sisi") {
		t.Fatal("expected side-specific rejection")
	}
}

func TestAjaibCorrection(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		raw, _ := io.ReadAll(r.Body)
		side := "RG"
		if calls == 2 {
			if !strings.Contains(string(raw), "sisi beli atau jual") {
				t.Error("missing validation feedback")
			}
			side = " jual "
		}
		draft := map[string]any{"is_trade_confirmation": true, "date": "2026-10-16", "rows": []map[string]string{{"ticker": " hrta ", "side": side, "shares": "2800", "price": "1615", "gross": "4522000"}}, "fees": []map[string]string{{"label": "Komisi", "sell": "6841"}, {"label": "PPH", "sell": "4522"}}, "sell_gross": "4522000", "sell_net": "4510637"}
		answer, _ := json.Marshal(draft)
		json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"parts": []any{map[string]string{"text": string(answer)}}}}}})
	}))
	defer srv.Close()
	old := geminiClient
	client := srv.Client()
	geminiClient = &http.Client{Transport: ajaibTransport(func(req *http.Request) (*http.Response, error) {
		u, _ := url.Parse(srv.URL)
		req.URL = u
		return client.Transport.RoundTrip(req)
	})}
	defer func() { geminiClient = old }()
	trades, err := extractValidatedPDF(t.Context(), "demo", "model", []byte("sample"))
	if err != nil || calls != 2 || len(trades) != 1 || trades[0].AmountDone != 4510637 || trades[0].Lot != 28 {
		t.Fatalf("calls=%d trades=%d err=%v", calls, len(trades), err)
	}
}

type ajaibTransport func(*http.Request) (*http.Response, error)

func (f ajaibTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
