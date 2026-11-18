package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

type gmailConnectionRepo struct {
	contract.AuthRepository
	user  models.Users
	saved bool
}

func (r *gmailConnectionRepo) FindByID(uint) (*models.Users, error) {
	user := r.user
	return &user, nil
}

func (r *gmailConnectionRepo) UpdateUser(user *models.Users) (*models.Users, error) {
	r.user, r.saved = *user, true
	return user, nil
}

type gmailConnectionTransport func(*http.Request) (*http.Response, error)

func (f gmailConnectionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGmailConnection(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("JWT_SECRET", "gmail-connection-test-secret")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("k", 32))
	config.Load()
	service := &AuthService{}

	loginURL, err := url.Parse(service.GetGoogleLoginURL("login-state"))
	if err != nil {
		t.Fatal(err)
	}
	if q := loginURL.Query(); q.Get("scope") != "email profile" || q.Has("prompt") || q.Has("access_type") {
		t.Fatalf("unexpected login parameters: %v", q)
	}
	start, err := service.StartGmailConnection(7)
	if err != nil {
		t.Fatal(err)
	}
	connectURL, err := url.Parse(start.URL)
	if err != nil {
		t.Fatal(err)
	}
	q := connectURL.Query()
	if q.Get("scope") != gmail.GmailReadonlyScope || q.Get("prompt") != "consent" ||
		q.Get("access_type") != "offline" || q.Get("state") != start.State ||
		q.Get("code_challenge") != oauth2.S256ChallengeFromVerifier(start.Verifier) || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("unexpected Gmail consent parameters: %v", q)
	}

	for _, tc := range []struct {
		name, scope, refresh, email, errorText string
		userID                                 uint
	}{
		{"different user", gmail.GmailReadonlyScope, "refresh", "investor@example.com", "tidak valid", 8},
		{"missing scope", "email", "refresh", "investor@example.com", "Izin baca", 7},
		{"missing refresh", gmail.GmailReadonlyScope, "", "investor@example.com", "akses offline", 7},
		{"different Gmail address", gmail.GmailReadonlyScope, "refresh", "other@example.com", "alamat email yang sama", 7},
		{"valid", gmail.GmailReadonlyScope, "refresh", "investor@example.com", "", 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &gmailConnectionRepo{user: models.Users{Id: 7, Email: "investor@example.com"}}
			service.AuthRepository = repo
			transport := gmailConnectionTransport(func(req *http.Request) (*http.Response, error) {
				var body map[string]any
				switch {
				case req.URL.String() == config.GetConfig().GoogleOAuthConfig.Endpoint.TokenURL:
					if tc.userID != 7 {
						t.Fatal("invalid state reached token exchange")
					}
					if err := req.ParseForm(); err != nil || req.Form.Get("code_verifier") != start.Verifier {
						t.Fatalf("missing PKCE verifier: %v", err)
					}
					body = map[string]any{"access_token": "access", "refresh_token": tc.refresh, "scope": tc.scope, "expires_in": 3600}
				case req.URL.Path == "/gmail/v1/users/me/profile":
					if tc.scope != gmail.GmailReadonlyScope || tc.refresh == "" {
						t.Fatal("profile requested without Gmail permission and refresh token")
					}
					if req.Header.Get("Authorization") != "Bearer access" {
						t.Fatal("profile request missing Gmail token")
					}
					body = map[string]any{"emailAddress": tc.email}
				default:
					t.Fatalf("unexpected Google request to %s", req.URL)
				}
				data, err := json.Marshal(body)
				if err != nil {
					return nil, err
				}
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(string(data))),
				}, nil
			})
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: transport})
			err := service.CompleteGmailConnection(ctx, tc.userID, dto.GmailConnectionRequest{
				Code: "code", State: start.State, Verifier: start.Verifier,
			})
			if tc.errorText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorText) || repo.saved {
					t.Fatalf("expected error containing %q and no save, got error %v, saved %t", tc.errorText, err, repo.saved)
				}
				return
			}
			if err != nil || !repo.saved || !repo.user.GmailSyncEnabled || repo.user.GmailRefreshToken == nil {
				t.Fatalf("expected Gmail to be enabled, got error %v, saved %t", err, repo.saved)
			}
			refresh, err := utils.Decrypt(*repo.user.GmailRefreshToken, config.GetConfig().EncryptionKey)
			if err != nil || refresh != tc.refresh {
				t.Fatal("expected encrypted refresh token to be saved")
			}
		})
	}
}
