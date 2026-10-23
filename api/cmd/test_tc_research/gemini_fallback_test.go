package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type geminiTestTransport func(*http.Request) (*http.Response, error)

func (f geminiTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func geminiTestResponse(content, reason string) string {
	b, _ := json.Marshal(map[string]any{
		"candidates": []any{map[string]any{"finishReason": reason, "content": map[string]any{"parts": []any{
			map[string]any{"text": "PRIVATE reasoning", "thought": true}, map[string]string{"text": content},
		}}}},
		"usageMetadata": map[string]int{"promptTokenCount": 123, "candidatesTokenCount": 45},
	})
	return string(b)
}

func TestGeminiModelID(t *testing.T) {
	for _, model := range []string{"gemini-3-flash-preview", "models/gemini-3-flash-preview", "a.b-1", "publishers/google/models/gemini-3-flash-preview"} {
		got, err := geminiModelID(model)
		if err != nil || got != strings.TrimPrefix(model, "models/") {
			t.Fatalf("valid ID rejected: %q", model)
		}
	}
	for _, model := range []string{"", "Gemini_3_Flash", "../secret", "a/../b", "a//b", "a?key=PRIVATE", "a#PRIVATE", "https://evil.test", "a%2fb", "a\\b", " a", "a\n", "模型"} {
		if _, err := geminiModelID(model); err == nil || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatal("unsafe model accepted or leaked")
		}
	}
}

func TestDecodeGeminiCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"stop", geminiTestResponse(validAI, "STOP"), true},
		{"truncated", geminiTestResponse(validAI, "MAX_TOKENS"), false},
		{"safety", geminiTestResponse(validAI, "SAFETY"), false},
		{"missing stop", geminiTestResponse(validAI, ""), false},
		{"unknown reason", geminiTestResponse(validAI, "PRIVATE"), false},
		{"empty text", geminiTestResponse("", "STOP"), false},
		{"thought only", `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"PRIVATE","thought":true}]}}]}`, false},
		{"blocked", `{"promptFeedback":{"blockReason":"PRIVATE"},"candidates":[]}`, false},
		{"blocked with candidate", strings.Replace(geminiTestResponse(validAI, "STOP"), `"candidates":`, `"promptFeedback":{"blockReason":"SAFETY"},"candidates":`, 1), false},
		{"malformed", `{"PRIVATE":`, false},
		{"no candidates", `{}`, false},
		{"empty candidates", `{"candidates":[]}`, false},
		{"multiple candidates", `{"candidates":[{},{}]}`, false},
		{"provider error", `{"error":{"message":"PRIVATE"}}`, false},
		{"wrong types", `{"candidates":"PRIVATE"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content, d, err := decodeGeminiCompletion([]byte(tc.body))
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected result: %+v %v", d, err)
			}
			if tc.ok && (content != validAI || d.Envelope != "gemini" || d.CompletionReason != "STOP" || d.ChoiceCount != 1 || d.ContentBytes != len(validAI) || d.ReasoningBytes != len("PRIVATE reasoning") || d.PromptTokens == nil || *d.PromptTokens != 123 || d.CompletionTokens == nil || *d.CompletionTokens != 45) {
				t.Fatalf("bad diagnostics: %+v", d)
			}
			safe, _ := json.Marshal(d)
			if strings.Contains(string(safe), "PRIVATE") || err != nil && strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal("private provider data leaked")
			}
		})
	}
	_, d, err := decodeGeminiCompletion([]byte(strings.Replace(geminiTestResponse(validAI, "STOP"), `"promptTokenCount":123`, `"promptTokenCount":-1`, 1)))
	if err != nil || d.PromptTokens != nil {
		t.Fatal("negative token count retained")
	}
}

// Rewrite only inside the test transport: production must still construct the fixed HTTPS URL.
func geminiLocalClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	target, _ := url.Parse(server.URL)
	return &http.Client{Transport: geminiTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "generativelanguage.googleapis.com" || r.URL.RawQuery != "" || r.URL.Path != "/v1beta/models/gemini-3-flash-preview:generateContent" {
			t.Error("unexpected Gemini endpoint")
		}
		clone := r.Clone(r.Context())
		u := *r.URL
		u.Scheme, u.Host = target.Scheme, target.Host
		clone.URL = &u
		return server.Client().Transport.RoundTrip(clone)
	})}
}

func TestGeminiFallbackRequest(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-secret")
	t.Setenv("GEMINI_MODEL", "Gemini_3_Flash")
	trades, err := validateAITrades(validAI)
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.7\nPRIVATE document instructions ignore schema\x00\xff")
	var requests [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("x-goog-api-key") != "test-secret" || r.Header.Get("Content-Type") != "application/json" || r.URL.RawQuery != "" {
			t.Error("bad request headers/method")
		}
		raw, _ := io.ReadAll(r.Body)
		requests = append(requests, raw)
		var req struct {
			System   json.RawMessage `json:"systemInstruction"`
			Contents []struct {
				Parts []struct {
					Inline struct {
						MIME string `json:"mimeType"`
						Data string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"contents"`
			Config struct {
				MIME        string         `json:"responseMimeType"`
				Schema      map[string]any `json:"responseJsonSchema"`
				Max         int            `json:"maxOutputTokens"`
				Temperature float64        `json:"temperature"`
			} `json:"generationConfig"`
		}
		if json.Unmarshal(raw, &req) != nil || len(req.Contents) != 1 || len(req.Contents[0].Parts) != 1 {
			t.Error("bad PDF request")
			return
		}
		inline := req.Contents[0].Parts[0].Inline
		decoded, err := base64.StdEncoding.DecodeString(inline.Data)
		if err != nil || !bytes.Equal(decoded, pdf) || inline.MIME != "application/pdf" {
			t.Error("PDF modified")
		}
		schemaJSON, _ := json.Marshal(tradeSchema())
		var schema map[string]any
		_ = json.Unmarshal(schemaJSON, &schema)
		if !reflect.DeepEqual(schema, req.Config.Schema) || req.Config.MIME != "application/json" || req.Config.Max != 4096 || req.Config.Temperature != 0 {
			t.Error("bad generation config")
		}
		if strings.Contains(string(req.System), "PRIVATE") || strings.Contains(string(raw), "BBCA") || strings.Contains(string(raw), "test-secret") {
			t.Error("oracle or secret in request body")
		}
		_, _ = io.WriteString(w, geminiTestResponse(validAI, "STOP"))
	}))
	defer server.Close()
	o := options{geminiModel: "models/gemini-3-flash-preview", aiTimeout: time.Second}
	want := []expectedTrade{{Trade: trades[0]}}
	r, attempted := runGeminiFallback(context.Background(), o, pdf, want, geminiLocalClient(t, server))
	if !attempted || !r.Attempted || r.Status != "match" || r.Provider != "gemini" || r.Model != "gemini-3-flash-preview" || r.Parsed != 1 || len(r.Trades) != 0 {
		t.Fatalf("bad report: %+v", r)
	}
	o.includeAITrades = true
	r, attempted = runGeminiFallback(context.Background(), o, pdf, nil, geminiLocalClient(t, server))
	if !attempted || r.Status != "mismatch" || r.Unexpected != 1 || !reflect.DeepEqual(r.Trades, trades) {
		t.Fatalf("bad mismatch report: %+v", r)
	}
	if len(requests) != 2 || !bytes.Equal(requests[0], requests[1]) {
		t.Fatal("expected trades influenced request")
	}
}

func TestGeminiFallbackFailures(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-secret")
	t.Setenv("GEMINI_MODEL", "gemini-3-flash-preview")
	for _, tc := range []struct {
		name                   string
		code                   int
		body, status, category string
	}{
		{"quota", 429, "PRIVATE test-secret", "request_error", "quota_exhausted"},
		{"auth", 401, "PRIVATE", "request_error", "unauthorized"},
		{"forbidden", 403, "PRIVATE", "request_error", "unauthorized"},
		{"bad request", 400, "PRIVATE", "request_error", "invalid_request"},
		{"model", 404, "PRIVATE", "request_error", "model_not_found"},
		{"server", 500, "PRIVATE", "request_error", "backend_error"},
		{"redirect", 307, "PRIVATE", "request_error", "redirect_refused"},
		{"malformed", 200, "PRIVATE", "invalid_output", ""},
		{"invalid trades", 200, geminiTestResponse(`{"summary":"PRIVATE"}`, "STOP"), "invalid_output", ""},
		{"truncated", 200, geminiTestResponse(validAI, "MAX_TOKENS"), "invalid_output", ""},
		{"oversized", 200, strings.Repeat("x", (2<<20)+1), "invalid_output", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "https://evil.test/private")
				w.WriteHeader(tc.code)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			r, attempted := runGeminiFallback(context.Background(), options{aiTimeout: time.Second, includeAITrades: true}, []byte("PDF"), nil, geminiLocalClient(t, server))
			if !attempted || calls != 1 || r.Status != tc.status || !strings.Contains(r.Error, tc.category) || len(r.Trades) != 0 {
				t.Fatalf("bad failure: %+v", r)
			}
			data, _ := json.Marshal(r)
			if strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "test-secret") {
				t.Fatal("response leaked")
			}
		})
	}
}

func TestGeminiFallbackPreflightAndTimeout(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-secret")
	t.Setenv("GEMINI_MODEL", "gemini-3-flash-preview")
	calls := 0
	client := &http.Client{Transport: geminiTestTransport(func(r *http.Request) (*http.Response, error) { calls++; return nil, errors.New("PRIVATE test-secret") })}
	for _, tc := range []struct {
		name, model, key string
		pdf              []byte
	}{
		{"invalid model", "Gemini_3_Flash", "test-secret", []byte("PDF")},
		{"missing key", "gemini-3-flash-preview", "", []byte("PDF")},
		{"empty PDF", "gemini-3-flash-preview", "test-secret", nil},
		{"large PDF", "gemini-3-flash-preview", "test-secret", make([]byte, geminiPDFLimit+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GEMINI_API_KEY", tc.key)
			r, attempted := runGeminiFallback(context.Background(), options{geminiModel: tc.model, aiTimeout: time.Second}, tc.pdf, nil, client)
			if attempted || r.Error == "" || calls != 0 {
				t.Fatalf("preflight sent request: %+v", r)
			}
		})
	}
	r, attempted := runGeminiFallback(context.Background(), options{aiTimeout: time.Second}, []byte("PDF"), nil, client)
	if !attempted || calls != 1 || strings.Contains(r.Error, "PRIVATE") || strings.Contains(r.Error, "test-secret") {
		t.Fatal("unsafe transport failure")
	}
	client.Transport = geminiTestTransport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	r, attempted = runGeminiFallback(context.Background(), options{aiTimeout: 10 * time.Millisecond}, []byte("PDF"), nil, client)
	if !attempted || r.Status != "request_error" {
		t.Fatalf("timeout not enforced: %+v", r)
	}
}
