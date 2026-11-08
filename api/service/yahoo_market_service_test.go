package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

const marketFixture = `{"chart":{"error":null,"result":[{
	"meta":{"symbol":"BBCA.JK","longName":"Bank Central Asia","currency":"IDR","exchangeName":"JKT",
	"regularMarketPrice":110,"previousClose":100,"regularMarketTime":1704276000,
	"regularMarketDayHigh":null,"regularMarketDayLow":0,"fiftyTwoWeekHigh":120,"regularMarketVolume":0},
	"timestamp":[1704103200,1704189600,null,1704276000,1704362400],
	"indicators":{"quote":[{"close":[90,null,95,110,115,120]}]}
}]}}`

func testMarketService(t *testing.T, handler http.HandlerFunc) *YahooService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	s := ImplYahooService(nil).(*YahooService)
	s.marketClient = server.Client()
	s.marketClient.Timeout = time.Second
	s.marketBaseURL = server.URL
	return s
}

func TestGetMarketParsingAndCache(t *testing.T) {
	var calls atomic.Int32
	s := testMarketService(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v8/finance/chart/BBCA.JK" || r.URL.Query().Get("range") != "1mo" || r.URL.Query().Get("interval") != "1d" {
			t.Errorf("unexpected Yahoo URL: %s", r.URL)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		fmt.Fprint(w, marketFixture)
	})
	data, err := s.GetMarket(context.Background(), "bbca")
	if err != nil {
		t.Fatal(err)
	}
	if data.Ticker != "BBCA" || data.Name != "Bank Central Asia" || data.Currency != "IDR" || data.Exchange != "JKT" {
		t.Fatalf("unexpected metadata: %+v", data)
	}
	if data.Price == nil || *data.Price != 110 || data.Change == nil || *data.Change != 10 || data.ChangePercent == nil || *data.ChangePercent != 10 {
		t.Fatalf("unexpected prices: %+v", data)
	}
	if data.DayHigh != nil || data.DayLow == nil || *data.DayLow != 0 || data.FiftyTwoWeekLow != nil || data.Volume == nil || *data.Volume != 0 {
		t.Fatalf("null and zero values not preserved: %+v", data)
	}
	if len(data.History) != 3 || data.History[0].Close != 90 || data.History[1].Time != 1704276000 {
		t.Fatalf("unexpected history: %+v", data.History)
	}
	if _, err := time.Parse(time.RFC3339Nano, data.FetchedAt); err != nil {
		t.Fatalf("invalid fetched_at: %q", data.FetchedAt)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"day_high":null`) || !strings.Contains(string(encoded), `"day_low":0`) {
		t.Fatalf("unexpected JSON: %s", encoded)
	}
	fetchedAt := data.FetchedAt
	*data.Price = 999
	data.History[0].Close = 999
	cached, err := s.GetMarket(context.Background(), "BBCA")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || *cached.Price != 110 || cached.History[0].Close != 90 || cached.FetchedAt != fetchedAt {
		t.Fatalf("cache miss or cache mutated: %+v; calls=%d", cached, calls.Load())
	}

	s.marketMu.Lock()
	entry := s.marketCache["BBCA"]
	entry.fetchedAt = time.Now().Add(-marketCacheTTL - time.Second)
	s.marketCache["BBCA"] = entry
	s.marketMu.Unlock()
	if _, err := s.GetMarket(context.Background(), "BBCA"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("expired cache did not refetch")
	}
}

func TestGetMarketValidation(t *testing.T) {
	var calls atomic.Int32
	s := testMarketService(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		symbol := strings.TrimPrefix(r.URL.Path, "/v8/finance/chart/")
		fmt.Fprintf(w, `{"chart":{"result":[{"meta":{"symbol":%q}}]}}`, symbol)
	})
	for _, ticker := range []string{"", "BBCA.JK", "BB-CA", " BBCA", "BBCA ", "a/b", "ABCDEFGHIJKLM", "é", "../BBCA"} {
		t.Run(ticker, func(t *testing.T) {
			_, err := s.GetMarket(context.Background(), ticker)
			if err == nil || errs.GetStatusCode(err) != http.StatusBadRequest {
				t.Fatalf("expected 400, got %v", err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("invalid ticker reached upstream")
	}
	for _, ticker := range []string{"a", "A1", "abcdefghijkl"} {
		if _, err := s.GetMarket(context.Background(), ticker); err != nil {
			t.Errorf("valid ticker %q rejected: %v", ticker, err)
		}
	}
}

func TestGetMarketUpstreamErrorsNotCached(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   int
	}{
		{"rate limited", 429, "slow down", 429},
		{"not found", 404, "missing", 404},
		{"unauthorized upstream", 401, "auth", 502},
		{"forbidden upstream", 403, "forbidden", 502},
		{"server error", 500, "down", 502},
		{"unexpected status", 204, "", 502},
		{"invalid JSON", 200, "not JSON", 502},
		{"missing chart", 200, `{}`, 502},
		{"empty results", 200, `{"chart":{"result":[]}}`, 502},
		{"null result", 200, `{"chart":{"result":[null]}}`, 502},
		{"wrong symbol", 200, `{"chart":{"result":[{"meta":{"symbol":"OTHER.JK"}}]}}`, 502},
		{"chart error", 200, `{"chart":{"error":{"code":"Not Found","description":"No data"}}}`, 404},
		{"chart rate limit", 200, `{"chart":{"error":{"code":"Too Many Requests","description":"Slow down"}}}`, 429},
		{"chart service error", 200, `{"chart":{"error":{"code":"Internal Error","description":"Unavailable"}}}`, 502},
		{"oversized response", 200, strings.Repeat(" ", marketBodyMax+1), 502},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			s := testMarketService(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			for i := 0; i < 2; i++ {
				_, err := s.GetMarket(context.Background(), "BBCA")
				if err == nil || errs.GetStatusCode(err) != tc.want {
					t.Fatalf("want status %d, got %v", tc.want, err)
				}
			}
			if calls.Load() != 2 || len(s.marketCache) != 0 {
				t.Fatal("failure was cached")
			}
		})
	}
}

func TestGetMarketTimeout(t *testing.T) {
	s := testMarketService(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	s.marketClient.Timeout = 20 * time.Millisecond
	_, err := s.GetMarket(context.Background(), "BBCA")
	if err == nil || errs.GetStatusCode(err) != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %v", err)
	}
	if len(s.marketCache) != 0 {
		t.Fatal("timeout was cached")
	}
}

func TestGetMarketCacheBoundAndConcurrency(t *testing.T) {
	s := testMarketService(t, func(w http.ResponseWriter, r *http.Request) {
		symbol := strings.TrimPrefix(r.URL.Path, "/v8/finance/chart/")
		fmt.Fprintf(w, `{"chart":{"result":[{"meta":{"symbol":%q,"regularMarketPrice":1}}]}}`, symbol)
	})
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := worker; i < marketCacheMax+16; i += 8 {
				if _, err := s.GetMarket(context.Background(), fmt.Sprintf("T%d", i)); err != nil {
					t.Error(err)
				}
			}
		}(worker)
	}
	wg.Wait()
	s.marketMu.Lock()
	if len(s.marketCache) != marketCacheMax {
		t.Errorf("cache size = %d; want %d", len(s.marketCache), marketCacheMax)
	}
	s.marketMu.Unlock()

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := s.GetMarket(context.Background(), "SAME")
			if err != nil {
				t.Error(err)
				return
			}
			*data.Price = 2
		}()
	}
	wg.Wait()
	data, err := s.GetMarket(context.Background(), "SAME")
	if err != nil || *data.Price != 1 {
		t.Fatalf("cache mutated concurrently: data=%+v err=%v", data, err)
	}
}

func TestParseMarketMissingAndNonfiniteValues(t *testing.T) {
	var response dto.YahooMarketResponse
	if err := json.Unmarshal([]byte(`{"chart":{"result":[{"meta":{"symbol":"BBCA.JK","regularMarketPrice":null,"previousClose":null},"timestamp":[1,2,3,4,5],"indicators":{"quote":[{"close":[null,0,3,4,5]}]}}]}}`), &response); err != nil {
		t.Fatal(err)
	}
	result := response.Chart.Result[0]
	nan, inf := math.NaN(), math.Inf(1)
	result.Indicators.Quote[0].Close[2] = &nan
	result.Indicators.Quote[0].Close[3] = &inf
	result.Meta.RegularMarketDayHigh = &inf
	data := parseMarket("BBCA", result)
	if data.Price != nil || data.PreviousClose != nil || data.Change != nil || data.ChangePercent != nil || data.DayHigh != nil || data.MarketTime != nil || data.Volume != nil {
		t.Fatalf("absent/nonfinite fields became numeric values: %+v", data)
	}
	if len(data.History) != 2 || data.History[0].Close != 0 || data.History[1].Close != 5 {
		t.Fatalf("invalid history: %+v", data.History)
	}
	if _, err := json.Marshal(data); err != nil {
		t.Fatalf("response is not JSON-safe: %v", err)
	}
	data = parseMarket("BBCA", dto.YahooMarketResult{})
	if data.History == nil || len(data.History) != 0 {
		t.Fatal("missing history must serialize as []")
	}
}

func TestParseMarketPreviousClose(t *testing.T) {
	// The quote is Jan 3 in Jakarta but Jan 2 in UTC. The Jan 3 daily bar
	// must not be used as the prior day's close.
	var response dto.YahooMarketResponse
	if err := json.Unmarshal([]byte(`{"chart":{"result":[{
		"meta":{"symbol":"BBCA.JK","regularMarketPrice":120,"chartPreviousClose":50,
		"regularMarketTime":1704216600,"exchangeTimezoneName":"Asia/Jakarta","gmtoffset":25200},
		"timestamp":[1704247200,1704160800,1704074400],
		"indicators":{"quote":[{"close":[120,100,90]}]}
	}]}}`), &response); err != nil {
		t.Fatal(err)
	}
	result := response.Chart.Result[0]
	data := parseMarket("BBCA", result)
	if data.PreviousClose == nil || *data.PreviousClose != 100 || *data.Change != 20 || *data.ChangePercent != 20 {
		t.Fatalf("incorrect daily previous close: %+v", data)
	}
	zero := 0.0
	result.Meta.PreviousClose = &zero
	data = parseMarket("BBCA", result)
	if *data.PreviousClose != 0 || *data.Change != 120 || data.ChangePercent != nil {
		t.Fatalf("zero previous close must not divide by zero: %+v", data)
	}
}
