package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
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
	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()
	var key, model string
	if !localPDFEnabled() {
		if err := godotenv.Load(os.Getenv("AJAIB_TEST_ENV")); err != nil {
			t.Fatal(err)
		}
		key, model, err = geminiConfig()
		if err != nil {
			t.Fatal(err)
		}
	}
	trades, err := extractValidatedWithProvider(ctx, key, model, pdf, localPDFEnabled(), "Ajaib Sekuritas")
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

func TestAjaibWrongCellsStayPending(t *testing.T) {
	const original = "Ajaib\n2026-10-16\nSELL\nTicker | Harga | Jumlah lembar | Bruto jual\nHRTA | 1615 | 2800 | 4522000.00\nKomisi Broker | 6841\nPPH | 4522\nTotal Terima | 4510637"
	valid := ajaibLocations{Ticker: 1, Price: 2, Shares: 3, Gross: 4, Net: 8}
	for _, test := range []struct {
		name, doc string
		edit      func(*ajaibLocations)
		valid     bool
	}{
		{"printed", original, nil, true},
		{"wrong tax", strings.Replace(original, "4522\n", "522\n", 1), nil, false},
		{"wrong column", original, func(p *ajaibLocations) { p.Price, p.Shares = p.Shares, p.Price }, false},
		{"wrong total line", original, func(p *ajaibLocations) { p.Net = 7 }, false},
		{"missing fee", strings.Replace(original, "PPH | 4522\n", "", 1), func(p *ajaibLocations) { p.Net = 7 }, false},
		{"ambiguous side", strings.Replace(original, "SELL", "SELL / BUY", 1), nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := valid
			if test.edit != nil {
				test.edit(&p)
			}
			table, err := parseAjaibTable(test.doc)
			if err == nil {
				var d *dto.TradeDraft
				d, err = table.draft(p)
				if err == nil {
					_, err = validateTradeDraft(*d)
				}
			}
			if (err == nil) != test.valid {
				t.Fatalf("accepted=%v err=%v", err == nil, err)
			}
		})
	}
}

func TestLocalAjaibReorderedAndCorruptPDF(t *testing.T) {
	if os.Getenv("AJAIB_ANON_VARIANT") == "" || !localPDFEnabled() {
		t.Skip("opt-in local Ollama test")
	}
	// Synthetic second layout with a different date, security, quantity and fee.
	for _, tc := range []struct {
		name, tax string
		valid     bool
	}{{"reordered", "25", true}, {"incorrect_tax", "5", false}} {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{"Ajaib Sekuritas - Trade confirmation", "2025-08-12", "SELL", "Jumlah lembar | Bruto jual | Ticker | Harga", "300 | 300000 | ABCD | 1000", "Komisi | 300", "PPH | " + tc.tax, "Total Terima | 299675"}
			var ps strings.Builder
			ps.WriteString("%!PS\n/Courier findfont 11 scalefont setfont\n")
			for i, line := range lines {
				fmt.Fprintf(&ps, "30 %d moveto (%s) show\n", 720-i*20, line)
			}
			ps.WriteString("showpage\n")
			cmd := exec.CommandContext(t.Context(), "ps2pdf", "-", "-")
			cmd.Stdin = strings.NewReader(ps.String())
			pdf, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			trades, err := extractValidatedWithProvider(t.Context(), "", "", pdf, true, "Ajaib Sekuritas")
			if !tc.valid {
				if err == nil || !strings.Contains(err.Error(), "Total SELL") || len(trades) != 0 {
					t.Fatalf("expected rejection on printed net; got rows=%d err=%v", len(trades), err)
				}
				return
			}
			if err != nil || len(trades) != 1 {
				t.Fatalf("rows=%d err=%v", len(trades), err)
			}
			r := trades[0]
			if r.Ticker != "ABCD" || r.Lot != 3 || r.Price != 1000 || r.Tax != 325 || r.AmountDone != 299675 {
				t.Fatal("values differ from printed cells")
			}
		})
	}
}

func TestAjaibOriginalPanelFallback(t *testing.T) {
	path := os.Getenv("AJAIB_SAMPLE_PDF")
	if path == "" || !localPDFEnabled() {
		t.Skip("requires original sample and local model")
	}
	pdf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := extractValidatedWithProvider(t.Context(), "", "", pdf, true, "Ajaib Sekuritas")
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	r := rows[0]
	if r.Ticker != "HRTA" || r.Type != "SELL" || r.Lot != 28 || r.Tax != 11363 || r.AmountDone != 4510637 {
		t.Fatal("values differ from original TC")
	}
}

func TestAjaibPanelReconciliation(t *testing.T) {
	const heading = "Tipe  Efek  Harga  Jumlah  Total Beli"
	layout := "Tgl. Transaksi : 10 October 2025\nTotal Bayar : Rp 10010.00\n" + fmt.Sprintf("%-50s%s", heading, "Tipe  Efek  Harga  Jumlah  Total Jual") + "\nRG  AAAA  100  100  10000\nTotal Beli : 10000\nTotal Transaksi : 10000.00\nKomisi : (10)\nTotal Keseluruhan : 10010.00\nCatatan\n"
	for _, c := range []struct {
		name, doc string
		valid     bool
	}{
		{"buy", layout, true},
		{"conflicting gross", strings.Replace(layout, "Total Transaksi : 10000", "Total Transaksi : 11000", 1), false},
		{"conflicting net", strings.Replace(layout, "Total Keseluruhan : 10010", "Total Keseluruhan : 10020", 1), false},
		{"both sides", strings.Replace(layout, "RG  AAAA  100  100  10000", fmt.Sprintf("%-50s%s", "RG AAAA 100 100 10000", "RG BBBB 100 200 20000"), 1), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			table, err := parseAjaibTable(c.doc)
			if c.valid && err == nil {
				d, e := table.draft(ajaibLocations{Ticker: 2, Price: 3, Shares: 4, Gross: 5, Net: table.netLine})
				if e != nil {
					t.Fatal(e)
				}
				rows, e := validateTradeDraft(*d)
				if e != nil || len(rows) != 1 || rows[0].AmountDone != 10010 {
					t.Fatal("buy total mismatch", e)
				}
			}
			if (err == nil) != c.valid {
				t.Fatalf("valid=%v err=%v", c.valid, err)
			}
		})
	}
}
