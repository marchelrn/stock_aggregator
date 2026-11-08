// Command test_gmail_sync bootstraps OAuth for test_tc_research --gmail.
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const tokenFile = ".gmail_token.json"

func main() {
	if err := run(context.Background(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out io.Writer) error {
	// Support invocation from either api or api/cmd/test_gmail_sync.
	if err := godotenv.Load(".env"); err != nil {
		if !os.IsNotExist(err) {
			return errors.New("cannot load .env")
		}
		if err := godotenv.Load("../../.env"); err != nil && !os.IsNotExist(err) {
			return errors.New("cannot load api .env")
		}
	}
	config := &oauth2.Config{
		ClientID: os.Getenv("GOOGLE_CLIENT_ID"), ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL: "http://localhost:9090/callback",
		Scopes:      []string{"https://www.googleapis.com/auth/gmail.readonly"},
		Endpoint:    google.Endpoint,
	}
	if config.ClientID == "" || config.ClientSecret == "" {
		return errors.New("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required in api .env or environment")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 30 * time.Second})
	token, err := loadToken(tokenFile)
	if err == nil {
		refreshCtx, refreshCancel := context.WithTimeout(ctx, 30*time.Second)
		token, err = config.TokenSource(refreshCtx, token).Token()
		refreshCancel()
	}
	if err != nil {
		fmt.Fprintln(out, "Browser login required.")
		token, err = authenticate(ctx, config, out)
		if err != nil {
			return err
		}
	}
	if err := saveToken(tokenFile, token); err != nil {
		return errors.New("cannot save private Gmail token file")
	}
	fmt.Fprintln(out, "Gmail token ready in .gmail_token.json for test_tc_research --gmail. No messages fetched or parsed.")
	return nil
}

type callbackResult struct {
	code string
	err  error
}

func callbackHandler(state string, results chan<- callbackResult) http.Handler {
	var once sync.Once
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		if state == "" || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			return
		}
		result := callbackResult{code: q.Get("code")}
		if q.Get("error") != "" {
			result = callbackResult{err: errors.New("OAuth authorization denied")}
		} else if result.code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			return
		}
		once.Do(func() {
			// A buffered, nonblocking handoff also tolerates repeated callbacks.
			select {
			case results <- result:
			default:
			}
		})
		fmt.Fprintln(w, "Authorization received. Return to the terminal for the result; this tab can be closed.")
	})
}

func authenticate(ctx context.Context, config *oauth2.Config, out io.Writer) (*oauth2.Token, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, errors.New("cannot generate OAuth state")
	}
	state := hex.EncodeToString(random[:])
	verifier := oauth2.GenerateVerifier()
	listener, err := net.Listen("tcp4", "127.0.0.1:9090")
	if err != nil {
		return nil, errors.New("cannot listen for OAuth callback on 127.0.0.1:9090")
	}
	results := make(chan callbackResult, 1)
	server := &http.Server{Handler: callbackHandler(state, results), ReadHeaderTimeout: 5 * time.Second}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if server.Shutdown(shutdownCtx) != nil {
			_ = server.Close()
		}
	}()
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	fmt.Fprintln(out, "Open this URL in your browser to authorize Gmail readonly access:")
	fmt.Fprintln(out, config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.S256ChallengeOption(verifier)))
	select {
	case <-ctx.Done():
		return nil, errors.New("OAuth login canceled or timed out")
	case <-serveErrors:
		return nil, errors.New("OAuth callback listener stopped")
	case result := <-results:
		if result.err != nil {
			return nil, result.err
		}
		token, err := config.Exchange(ctx, result.code, oauth2.VerifierOption(verifier))
		if err != nil || !token.Valid() {
			// Provider errors can contain authorization codes or token responses.
			return nil, errors.New("OAuth token exchange failed")
		}
		return token, nil
	}
}

func saveToken(path string, token *oauth2.Token) error {
	if token == nil || !token.Valid() {
		return errors.New("invalid OAuth token")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	// OpenFile's mode only applies to new files; tighten existing files too.
	if err := f.Chmod(0600); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(token); err != nil {
		return err
	}
	return f.Close()
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var token oauth2.Token
	if err := json.NewDecoder(io.LimitReader(f, 1<<20)).Decode(&token); err != nil {
		return nil, errors.New("invalid OAuth token file")
	}
	if token.AccessToken == "" {
		return nil, errors.New("missing OAuth access token")
	}
	return &token, nil
}
