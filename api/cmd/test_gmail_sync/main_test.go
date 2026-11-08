package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestCallbackValidation(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		status       int
		result       bool
		denied       bool
	}{
		{"missing state", "/callback?code=private-code", 400, false, false},
		{"wrong state", "/callback?state=wrong&code=private-code", 400, false, false},
		{"missing code", "/callback?state=expected", 400, false, false},
		{"wrong path", "/other?state=expected&code=private-code", 404, false, false},
		{"denied", "/callback?state=expected&error=private-error", 200, true, true},
		{"valid", "/callback?state=expected&code=private-code", 200, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results := make(chan callbackResult, 1)
			handler := callbackHandler("expected", results)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.target, nil))
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if strings.Contains(w.Body.String(), "private-") {
				t.Fatal("callback response leaked sensitive data")
			}
			select {
			case result := <-results:
				if !tc.result || (result.err != nil) != tc.denied {
					t.Fatal("unexpected callback result")
				}
			default:
				if tc.result {
					t.Fatal("missing callback result")
				}
			}
		})
	}
}

func TestCallbackDuplicateDoesNotBlock(t *testing.T) {
	results := make(chan callbackResult, 1)
	handler := callbackHandler("expected", results)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 3 {
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/callback?state=expected&code=secret", nil))
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("duplicate callback blocked")
	}
	if len(results) != 1 {
		t.Fatal("expected exactly one callback result")
	}
}

func TestTokenSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gmail_token.json")
	// Existing files must also have their permissions tightened.
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 2048)), 0644); err != nil {
		t.Fatal(err)
	}
	want := &oauth2.Token{AccessToken: "private-access", RefreshToken: "private-refresh", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour).UTC()}
	if err := saveToken(path, want); err != nil {
		t.Fatal("could not save token")
	}
	got, err := loadToken(path)
	if err != nil {
		t.Fatal("could not load token")
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || !got.Expiry.Equal(want.Expiry) {
		t.Fatal("token did not round-trip")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows permission bits do not model POSIX owner-only access.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("token permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestInvalidTokenFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.json")
	if _, err := loadToken(path); err == nil {
		t.Fatal("missing token accepted")
	}
	for _, data := range []string{"not-json-private-secret", "{}"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadToken(path); err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatal("invalid token accepted or leaked")
		}
	}
	if err := saveToken(path, nil); err == nil {
		t.Fatal("nil token accepted")
	}
}
