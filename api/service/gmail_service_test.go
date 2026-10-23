package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/service/parser"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type gmailLegacyParser struct {
	calls int
	data  []byte
	err   error
	empty bool
}

func (p *gmailLegacyParser) Parse(data []byte) ([]dto.ParsedTrade, error) {
	p.calls++
	p.data = append([]byte(nil), data...)
	if p.err != nil || p.empty {
		return nil, p.err
	}
	return []dto.ParsedTrade{{}}, nil
}
func (*gmailLegacyParser) BrokerName() string { return "test" }

type gmailMetadataParser struct {
	gmailLegacyParser
	metadataCalls int
	metadata      parser.EmailMetadata
	ctx           context.Context
}

func (p *gmailMetadataParser) ParseEmail(ctx context.Context, data []byte, meta parser.EmailMetadata) ([]dto.ParsedTrade, error) {
	p.metadataCalls++
	p.metadata = meta
	p.ctx = ctx
	p.data = append([]byte(nil), data...)
	if p.err != nil || p.empty {
		return nil, p.err
	}
	return []dto.ParsedTrade{{}}, nil
}

type gmailSelectorParser struct {
	gmailMetadataParser
	selectors     []parser.EmailMetadata
	selectorErr   error
	selectorCalls int
	selectorCtx   context.Context
	parseEmail    func(parser.EmailMetadata) ([]dto.ParsedTrade, error)
}

func (p *gmailSelectorParser) EmailSelectors(ctx context.Context) ([]parser.EmailMetadata, error) {
	p.selectorCalls++
	p.selectorCtx = ctx
	return p.selectors, p.selectorErr
}

func (p *gmailSelectorParser) ParseEmail(ctx context.Context, data []byte, meta parser.EmailMetadata) ([]dto.ParsedTrade, error) {
	trades, err := p.gmailMetadataParser.ParseEmail(ctx, data, meta)
	if p.parseEmail != nil {
		return p.parseEmail(meta)
	}
	return trades, err
}

type gmailAuthRepo struct {
	contract.AuthRepository
	user    *models.Users
	updates int
	err     error
}

func (r *gmailAuthRepo) FindByID(uint) (*models.Users, error) { return r.user, nil }
func (r *gmailAuthRepo) UpdateUser(user *models.Users) (*models.Users, error) {
	r.updates++
	if r.err != nil {
		return nil, r.err
	}
	copy := *user
	r.user = &copy
	return &copy, nil
}

func fakeGmail(t *testing.T, handler http.HandlerFunc) *gmail.Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := gmail.NewService(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func gmailJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode fake Gmail response: %v", err)
	}
}

func TestGmailProcessMIMEParts(t *testing.T) {
	data := []byte("%PDF-test\xfb\xff")
	for _, tc := range []struct {
		name    string
		root    bool
		inline  bool
		padded  bool
		mimePDF bool
	}{
		{name: "root inline raw", root: true, inline: true},
		{name: "nested inline padded", inline: true, padded: true},
		{name: "nested downloaded raw"},
		{name: "root downloaded padded", root: true, padded: true},
		{name: "MIME PDF without filename", root: true, inline: true, mimePDF: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded := base64.RawURLEncoding.EncodeToString(data)
			if tc.padded {
				encoded = base64.URLEncoding.EncodeToString(data)
			}
			downloads := 0
			client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
				downloads++
				if !strings.HasSuffix(r.URL.Path, "/messages/message/attachments/attachment") {
					t.Errorf("unexpected URL: %s", r.URL)
				}
				gmailJSON(t, w, &gmail.MessagePartBody{Data: encoded})
			})
			part := &gmail.MessagePart{Filename: "Confirmation.PDF", Body: &gmail.MessagePartBody{AttachmentId: "attachment"}}
			if tc.inline {
				part.Body.Data = encoded
			}
			if tc.mimePDF {
				part.Filename = ""
				part.MimeType = "application/pdf"
			}
			root := part
			if !tc.root {
				root = &gmail.MessagePart{Parts: []*gmail.MessagePart{nil, {MimeType: "multipart/mixed", Parts: []*gmail.MessagePart{{Filename: "ignored.txt"}, part}}}}
			}
			root.Headers = []*gmail.MessagePartHeader{nil, {Name: "fRoM", Value: "Broker <broker@example.com>"}, {Name: "SUBJECT", Value: "Trade confirmation actual subject"}}
			spy := &gmailMetadataParser{}
			service := &GmailServiceImpl{TradeParser: spy}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			trades, err := service.processMessageAttachments(ctx, client, &gmail.Message{Id: "message", Payload: root})
			if err != nil || len(trades) != 1 {
				t.Fatalf("trades=%v error=%v", trades, err)
			}
			want := parser.EmailMetadata{From: "Broker <broker@example.com>", Subject: "Trade confirmation actual subject", Filename: part.Filename}
			if spy.metadata != want || spy.metadataCalls != 1 || spy.calls != 0 || spy.ctx != ctx || string(spy.data) != string(data) {
				t.Fatalf("wrong parser invocation: %+v", spy)
			}
			if tc.inline && downloads != 0 || !tc.inline && downloads != 1 {
				t.Fatalf("downloads=%d inline=%v", downloads, tc.inline)
			}
		})
	}
}

func TestGmailProcessAttachmentErrors(t *testing.T) {
	valid := base64.RawURLEncoding.EncodeToString([]byte("PDF"))
	for _, tc := range []struct {
		name       string
		payload    *gmail.MessagePart
		nilMessage bool
		httpStatus int
		parserErr  error
		empty      bool
		want       string
	}{
		{name: "nil message", nilMessage: true, want: "no MIME payload"},
		{name: "nil payload", want: "no MIME payload"},
		{name: "no PDFs", payload: &gmail.MessagePart{}, want: "no PDF trades"},
		{name: "nil body", payload: &gmail.MessagePart{Filename: "file.pdf"}, want: "no body"},
		{name: "missing data", payload: &gmail.MessagePart{Filename: "file.pdf", Body: &gmail.MessagePartBody{}}, want: "no data"},
		{name: "bad inline base64", payload: &gmail.MessagePart{Filename: "file.pdf", Body: &gmail.MessagePartBody{Data: "%%%"}}, want: "decode PDF"},
		{name: "bad downloaded base64", payload: &gmail.MessagePart{Filename: "file.pdf", Body: &gmail.MessagePartBody{AttachmentId: "attachment"}}, want: "decode PDF"},
		{name: "download error", payload: &gmail.MessagePart{Filename: "file.pdf", Body: &gmail.MessagePartBody{AttachmentId: "attachment"}}, httpStatus: 403, want: "download PDF"},
		{name: "parser error no fallback", payload: &gmail.MessagePart{Filename: "file.pdf", Body: &gmail.MessagePartBody{Data: valid}}, parserErr: errors.New("template mismatch"), want: "template mismatch"},
		{name: "empty parser result", payload: &gmail.MessagePart{Filename: "file.pdf", Body: &gmail.MessagePartBody{Data: valid}}, empty: true, want: "no trades found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.httpStatus != 0 {
					http.Error(w, "download denied", tc.httpStatus)
					return
				}
				gmailJSON(t, w, &gmail.MessagePartBody{Data: "%%%"})
			})
			spy := &gmailMetadataParser{gmailLegacyParser: gmailLegacyParser{err: tc.parserErr, empty: tc.empty}}
			service := &GmailServiceImpl{TradeParser: spy}
			msg := &gmail.Message{Id: "message", Payload: tc.payload}
			if tc.nilMessage {
				msg = nil
			}
			trades, err := service.processMessageAttachments(context.Background(), client, msg)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(trades) != 0 {
				t.Fatalf("trades=%v error=%v; want %q", trades, err, tc.want)
			}
			if spy.calls != 0 {
				t.Fatal("metadata failure must not fall back to legacy Parse")
			}
		})
	}
}

func TestGmailSyncPaginationAndCheckpoint(t *testing.T) {
	previous := time.Now().Add(-24 * time.Hour)
	started := time.Now().Add(-time.Minute)
	user := &models.Users{LastGmailSyncAt: &previous}
	repo := &gmailAuthRepo{user: user}
	spy := &gmailLegacyParser{}
	service := &GmailServiceImpl{AuthRepository: repo, TradeParser: spy}
	lists, gets := 0, 0
	client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/messages") {
			lists++
			if r.URL.Query().Get("q") != "has:attachment filename:pdf after:"+previous.Format("2006/01/02") || r.URL.Query().Get("maxResults") != "50" {
				t.Errorf("unexpected search query: %s", r.URL)
			}
			response := &gmail.ListMessagesResponse{}
			if lists == 1 {
				for i := 0; i < 50; i++ {
					response.Messages = append(response.Messages, &gmail.Message{Id: fmt.Sprint(i)})
				}
				response.NextPageToken = "second"
			} else {
				if r.URL.Query().Get("pageToken") != "second" {
					t.Errorf("missing page token: %s", r.URL)
				}
				response.Messages = []*gmail.Message{{Id: "50"}}
			}
			gmailJSON(t, w, response)
			return
		}
		gets++
		if repo.updates != 0 {
			t.Error("checkpoint saved before all messages were processed")
		}
		gmailJSON(t, w, &gmail.Message{Payload: &gmail.MessagePart{Filename: "file.pdf", Body: &gmail.MessagePartBody{Data: "UERG"}}})
	})
	response, err := service.syncTradeConfirmations(context.Background(), client, user, started)
	if err != nil {
		t.Fatal(err)
	}
	if lists != 2 || gets != 51 || spy.calls != 51 || response.TradesFound != 51 || response.TradesSynced != 0 || !strings.Contains(response.Message, "belum disimpan") {
		t.Fatalf("lists=%d gets=%d calls=%d response=%+v", lists, gets, spy.calls, response)
	}
	if repo.updates != 1 || !user.LastGmailSyncAt.Equal(started) || !repo.user.LastGmailSyncAt.Equal(started) {
		t.Fatalf("checkpoint must equal sync start: updates=%d checkpoint=%v", repo.updates, user.LastGmailSyncAt)
	}
}

func TestGmailSyncFailureDoesNotAdvanceCheckpoint(t *testing.T) {
	for _, failure := range []string{"search", "second page", "message", "download", "parse", "empty", "nil payload", "checkpoint"} {
		t.Run(failure, func(t *testing.T) {
			previous := time.Now().Add(-24 * time.Hour)
			user := &models.Users{LastGmailSyncAt: &previous}
			repo := &gmailAuthRepo{user: user}
			spy := &gmailMetadataParser{}
			if failure == "parse" {
				spy.err = errors.New("parse failed")
			}
			spy.empty = failure == "empty"
			if failure == "checkpoint" {
				repo.err = errors.New("update failed")
			}
			service := &GmailServiceImpl{AuthRepository: repo, TradeParser: spy}
			client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/messages"):
					if failure == "search" || r.URL.Query().Get("pageToken") != "" {
						http.Error(w, "search failed", http.StatusForbidden)
						return
					}
					response := &gmail.ListMessagesResponse{Messages: []*gmail.Message{{Id: "message"}}}
					if failure == "second page" {
						response.NextPageToken = "next"
					}
					gmailJSON(t, w, response)
				case strings.Contains(r.URL.Path, "/attachments/"):
					http.Error(w, "download failed", http.StatusForbidden)
				default:
					if failure == "message" {
						http.Error(w, "get failed", http.StatusForbidden)
						return
					}
					msg := &gmail.Message{Id: "message", Payload: &gmail.MessagePart{Filename: "file.pdf", Body: &gmail.MessagePartBody{Data: "UERG"}}}
					if failure == "nil payload" {
						msg.Payload = nil
					}
					if failure == "download" {
						msg.Payload.Body = &gmail.MessagePartBody{AttachmentId: "attachment"}
					}
					gmailJSON(t, w, msg)
				}
			})
			response, err := service.syncTradeConfirmations(context.Background(), client, user, time.Now())
			if err == nil || response != nil {
				t.Fatalf("expected failure, got response=%+v error=%v", response, err)
			}
			wantUpdates := 0
			if failure == "checkpoint" {
				wantUpdates = 1
			}
			if repo.updates != wantUpdates || !user.LastGmailSyncAt.Equal(previous) || !repo.user.LastGmailSyncAt.Equal(previous) {
				t.Fatalf("failure advanced checkpoint: updates=%d user=%+v", repo.updates, user)
			}
			if spy.calls != 0 {
				t.Fatal("unexpected legacy fallback")
			}
		})
	}
}

func TestGmailSyncNoMessagesAndUpdateError(t *testing.T) {
	for _, updateError := range []bool{false, true} {
		t.Run(fmt.Sprint(updateError), func(t *testing.T) {
			user := &models.Users{}
			repo := &gmailAuthRepo{user: user}
			if updateError {
				repo.err = errors.New("checkpoint failed")
			}
			service := &GmailServiceImpl{AuthRepository: repo}
			client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
				gmailJSON(t, w, &gmail.ListMessagesResponse{})
			})
			started := time.Now().Add(-time.Minute)
			response, err := service.syncTradeConfirmations(context.Background(), client, user, started)
			if updateError {
				if err == nil || response != nil || user.LastGmailSyncAt != nil {
					t.Fatalf("response=%+v error=%v checkpoint=%v", response, err, user.LastGmailSyncAt)
				}
			} else if err != nil || response.TradesFound != 0 || response.TradesSynced != 0 || !user.LastGmailSyncAt.Equal(started) {
				t.Fatalf("response=%+v error=%v checkpoint=%v", response, err, user.LastGmailSyncAt)
			}
			if repo.updates != 1 {
				t.Fatalf("updates=%d", repo.updates)
			}
		})
	}
}

func TestGmailSelectorDiscovery(t *testing.T) {
	storeErr := errors.New("template store unavailable")
	for _, tc := range []struct {
		name       string
		selectors  []parser.EmailMetadata
		storeErr   error
		wantSender string
		wantErr    string
	}{
		{name: "wildcard sender", selectors: []parser.EmailMetadata{{Subject: "Execution Advice", Filename: "execution.pdf"}}},
		{name: "sender union", selectors: []parser.EmailMetadata{{From: "broker@example.com", Subject: "Execution Advice", Filename: "execution.pdf"}, {From: "other@example.com"}}, wantSender: ` {from:"broker@example.com" from:"other@example.com"}`},
		{name: "mixed wildcard", selectors: []parser.EmailMetadata{{From: "broker@example.com"}, {Subject: "Execution Advice", Filename: "execution.pdf"}}},
		{name: "duplicate sender", selectors: []parser.EmailMetadata{{From: "broker@example.com"}, {From: "broker@example.com"}}, wantSender: ` {from:"broker@example.com"}`},
		{name: "no active", wantErr: "no active"},
		{name: "store failure", storeErr: storeErr, wantErr: "template store unavailable"},
		{name: "display name", selectors: []parser.EmailMetadata{{From: "Broker <broker@example.com>"}}, wantErr: "invalid Gmail selector sender"},
		{name: "query injection", selectors: []parser.EmailMetadata{{From: `broker@example.com"} OR in:anywhere {from:"other@example.com`}}, wantErr: "invalid Gmail selector sender"},
		{name: "quoted mailbox", selectors: []parser.EmailMetadata{{From: `"broker"@example.com`}}, wantErr: "invalid Gmail selector sender"},
		{name: "CRLF", selectors: []parser.EmailMetadata{{From: "broker@example.com\r\n"}}, wantErr: "invalid Gmail selector sender"},
		{name: "invalid beside wildcard", selectors: []parser.EmailMetadata{{}, {From: "not a mailbox"}}, wantErr: "invalid Gmail selector sender"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			started := time.Now().Add(-time.Minute)
			user := &models.Users{LastGmailSyncAt: &previous}
			repo := &gmailAuthRepo{user: user}
			spy := &gmailSelectorParser{selectors: tc.selectors, selectorErr: tc.storeErr}
			spy.parseEmail = func(meta parser.EmailMetadata) ([]dto.ParsedTrade, error) {
				if meta.From != "Broker <broker@example.com>" || meta.Subject != "Daily Execution Advice 123" || meta.Filename != "2026_execution.pdf" {
					t.Errorf("actual metadata not passed: %+v", meta)
				}
				return []dto.ParsedTrade{{}}, nil
			}
			service := &GmailServiceImpl{AuthRepository: repo, TradeParser: spy}
			lists := 0
			client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.wantErr != "" {
					t.Error("selector failure must occur before Gmail requests")
				}
				if strings.HasSuffix(r.URL.Path, "/messages") {
					lists++
					want := "has:attachment filename:pdf after:2026/10/01" + tc.wantSender
					if got := r.URL.Query().Get("q"); got != want {
						t.Errorf("query=%q; want %q", got, want)
					}
					response := &gmail.ListMessagesResponse{}
					if lists == 1 {
						response.Messages = []*gmail.Message{{Id: "execution"}}
						response.NextPageToken = "next"
					} else if r.URL.Query().Get("pageToken") != "next" {
						t.Error("missing next page token")
					}
					gmailJSON(t, w, response)
					return
				}
				gmailJSON(t, w, &gmail.Message{Id: "execution", Payload: &gmail.MessagePart{
					Filename: "2026_execution.pdf", Body: &gmail.MessagePartBody{Data: "UERG"},
					Headers: []*gmail.MessagePartHeader{{Name: "From", Value: "Broker <broker@example.com>"}, {Name: "Subject", Value: "Daily Execution Advice 123"}},
				}})
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			response, err := service.syncTradeConfirmations(ctx, client, user, started)
			if spy.selectorCalls != 1 || spy.selectorCtx != ctx || spy.calls != 0 {
				t.Fatalf("selectorCalls=%d legacyCalls=%d context=%v", spy.selectorCalls, spy.calls, spy.selectorCtx)
			}
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || response != nil || lists != 0 || repo.updates != 0 || !user.LastGmailSyncAt.Equal(previous) {
					t.Fatalf("response=%+v error=%v lists=%d updates=%d checkpoint=%v", response, err, lists, repo.updates, user.LastGmailSyncAt)
				}
				if tc.storeErr != nil && !errors.Is(err, tc.storeErr) {
					t.Fatalf("store error not preserved: %v", err)
				}
				return
			}
			if err != nil || response.TradesFound != 1 || response.TradesSynced != 0 || lists != 2 || spy.metadataCalls != 1 || repo.updates != 1 || !user.LastGmailSyncAt.Equal(started) {
				t.Fatalf("response=%+v error=%v lists=%d parserCalls=%d updates=%d", response, err, lists, spy.metadataCalls, repo.updates)
			}
		})
	}
}

func TestGmailIneligiblePDFs(t *testing.T) {
	layoutErr := errors.New("eligible PDF has unknown layout")
	for _, tc := range []struct {
		name       string
		filenames  [][]string
		wantTrades int
		wantErr    bool
	}{
		{name: "all PDFs ineligible", filenames: [][]string{{"unrelated.pdf"}}},
		{name: "skip attachment then parse sibling", filenames: [][]string{{"unrelated.pdf", "eligible.pdf"}}, wantTrades: 1},
		{name: "skip email then parse next", filenames: [][]string{{"unrelated.pdf"}, {"eligible.pdf"}}, wantTrades: 1},
		{name: "eligible unknown layout aborts", filenames: [][]string{{"unrelated.pdf"}, {"eligible.pdf"}, {"unknown.pdf"}}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := time.Now().Add(-24 * time.Hour)
			started := time.Now().Add(-time.Minute)
			user := &models.Users{LastGmailSyncAt: &previous}
			repo := &gmailAuthRepo{user: user}
			spy := &gmailSelectorParser{selectors: []parser.EmailMetadata{{Subject: "Execution Advice"}}}
			spy.parseEmail = func(meta parser.EmailMetadata) ([]dto.ParsedTrade, error) {
				switch meta.Filename {
				case "unrelated.pdf":
					return nil, fmt.Errorf("selector mismatch: %w", parser.ErrEmailNotApplicable)
				case "unknown.pdf":
					return nil, layoutErr
				default:
					return []dto.ParsedTrade{{}}, nil
				}
			}
			service := &GmailServiceImpl{AuthRepository: repo, TradeParser: spy}
			expectedCalls := 0
			messages := make(map[string]*gmail.Message)
			list := &gmail.ListMessagesResponse{}
			for i, filenames := range tc.filenames {
				id := fmt.Sprint(i)
				list.Messages = append(list.Messages, &gmail.Message{Id: id})
				payload := &gmail.MessagePart{Headers: []*gmail.MessagePartHeader{{Name: "Subject", Value: "Execution Advice"}}}
				for _, filename := range filenames {
					expectedCalls++
					payload.Parts = append(payload.Parts, &gmail.MessagePart{Filename: filename, Body: &gmail.MessagePartBody{Data: "UERG"}})
				}
				messages[id] = &gmail.Message{Id: id, Payload: payload}
			}
			client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/messages") {
					gmailJSON(t, w, list)
					return
				}
				id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
				gmailJSON(t, w, messages[id])
			})
			response, err := service.syncTradeConfirmations(context.Background(), client, user, started)
			if spy.metadataCalls != expectedCalls || spy.calls != 0 || spy.selectorCalls != 1 {
				t.Fatalf("metadataCalls=%d want=%d legacyCalls=%d selectorCalls=%d", spy.metadataCalls, expectedCalls, spy.calls, spy.selectorCalls)
			}
			if tc.wantErr {
				if !errors.Is(err, layoutErr) || response != nil || repo.updates != 0 || !user.LastGmailSyncAt.Equal(previous) {
					t.Fatalf("response=%+v error=%v updates=%d checkpoint=%v", response, err, repo.updates, user.LastGmailSyncAt)
				}
				return
			}
			if err != nil || response.TradesFound != tc.wantTrades || response.TradesSynced != 0 || repo.updates != 1 || !user.LastGmailSyncAt.Equal(started) {
				t.Fatalf("response=%+v error=%v updates=%d checkpoint=%v", response, err, repo.updates, user.LastGmailSyncAt)
			}
		})
	}
}

func TestGmailTokenRefresh(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("ENCRYPTION_KEY", "01234567890123456789012345678901")
	cfg := config.GetConfig()
	if cfg == nil {
		config.Load()
		cfg = config.GetConfig()
	}
	original := *cfg
	t.Cleanup(func() { *cfg = original })
	cfg.EncryptionKey = []byte("01234567890123456789012345678901")

	for _, failure := range []string{"", "refresh", "save", "canceled"} {
		t.Run("failure="+failure, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if failure == "canceled" {
					t.Error("canceled refresh reached server")
				}
				if failure == "refresh" {
					http.Error(w, "refresh rejected", http.StatusBadRequest)
					return
				}
				if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh" {
					t.Errorf("unexpected token request: form=%v error=%v", r.Form, err)
				}
				gmailJSON(t, w, map[string]any{"access_token": "new-access", "token_type": "Bearer", "expires_in": 3600})
			}))
			defer server.Close()
			cfg.GoogleOAuthConfig = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: server.URL, AuthStyle: oauth2.AuthStyleInParams}}
			refresh, err := utils.Encrypt("refresh", cfg.EncryptionKey)
			if err != nil {
				t.Fatal(err)
			}
			previous := time.Now().Add(-24 * time.Hour)
			user := &models.Users{GmailRefreshToken: &refresh, LastGmailSyncAt: &previous}
			repo := &gmailAuthRepo{user: user}
			if failure == "save" {
				repo.err = errors.New("token save failed")
			}
			service := &GmailServiceImpl{AuthRepository: repo}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if failure == "canceled" {
				cancel()
			}
			client, err := service.getGmailClient(ctx, user)
			if failure != "" {
				if err == nil || client != nil {
					t.Fatalf("client=%v error=%v", client, err)
				}
				if failure == "canceled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("expected cancellation, got %v", err)
				}
			} else {
				if err != nil || client == nil || repo.updates != 1 || user.GmailAccessToken == nil {
					t.Fatalf("client=%v error=%v updates=%d", client, err, repo.updates)
				}
				token, err := utils.Decrypt(*user.GmailAccessToken, cfg.EncryptionKey)
				if err != nil || token != "new-access" {
					t.Fatalf("token=%q error=%v", token, err)
				}
			}
			if !user.LastGmailSyncAt.Equal(previous) {
				t.Fatal("token refresh changed sync checkpoint")
			}
		})
	}
}

func TestGmailSyncCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := &gmailAuthRepo{user: &models.Users{}}
	service := &GmailServiceImpl{AuthRepository: repo}
	client := fakeGmail(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request reached Gmail")
	})
	_, err := service.syncTradeConfirmations(ctx, client, repo.user, time.Now())
	if !errors.Is(err, context.Canceled) || repo.updates != 0 {
		t.Fatalf("error=%v updates=%d", err, repo.updates)
	}
}
