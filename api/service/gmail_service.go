package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"github.com/marchelrn/stock_api/service/parser"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GmailServiceImpl struct {
	AuthRepository  contract.AuthRepository
	TradeParser     TradeConfirmationParser
	TransactionRepo contract.TransactionRepository
	BrokerRepo      contract.BrokerRepository
}

// TradeConfirmationParser is the interface for parsing trade confirmation PDFs
type TradeConfirmationParser interface {
	Parse(pdfBytes []byte) ([]dto.ParsedTrade, error)
	BrokerName() string
}

// emailTradeConfirmationParser is optional so existing injected parsers remain compatible.
// A metadata parser failure must never fall back to Parse and bypass email matching.
type emailTradeConfirmationParser interface {
	ParseEmail(context.Context, []byte, parser.EmailMetadata) ([]dto.ParsedTrade, error)
}

type emailSelectorProvider interface {
	EmailSelectors(context.Context) ([]parser.EmailMetadata, error)
}

const gmailSyncTimeout = 5 * time.Minute

func ImplGmailService(authRepo contract.AuthRepository, parser TradeConfirmationParser, txRepo contract.TransactionRepository, brokerRepo contract.BrokerRepository) contract.GmailService {
	return &GmailServiceImpl{
		AuthRepository:  authRepo,
		TradeParser:     parser,
		TransactionRepo: txRepo,
		BrokerRepo:      brokerRepo,
	}
}

// getGmailClient creates a Gmail API client using the user's stored OAuth tokens
func (s *GmailServiceImpl) getGmailClient(ctx context.Context, user *models.Users) (*gmail.Service, error) {
	cfg := config.GetConfig()

	if user.GmailRefreshToken == nil {
		return nil, errs.BadRequest("Akun belum terhubung dengan Gmail. Silakan login ulang via Google.")
	}

	// Decrypt tokens
	refreshToken, err := utils.Decrypt(*user.GmailRefreshToken, cfg.EncryptionKey)
	if err != nil {
		return nil, errs.InternalServerError("Gagal mendekripsi token Gmail")
	}

	var accessToken string
	if user.GmailAccessToken != nil {
		accessToken, err = utils.Decrypt(*user.GmailAccessToken, cfg.EncryptionKey)
		if err != nil {
			log.Printf("Warning: failed to decrypt access token: %v", err)
			accessToken = ""
		}
	}

	var expiry time.Time
	if user.GmailTokenExpiry != nil {
		expiry = *user.GmailTokenExpiry
	}

	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       expiry,
		TokenType:    "Bearer",
	}

	// Reuse one token source so the initial refresh and subsequent requests share
	// the request deadline and do not independently refresh the same token.
	tokenSource := cfg.GoogleOAuthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("refresh Gmail token: %w", err)
	}
	if newToken.AccessToken != token.AccessToken {
		encAccessToken, err := utils.Encrypt(newToken.AccessToken, cfg.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt Gmail token: %w", err)
		}
		updated := *user
		updated.GmailAccessToken = &encAccessToken
		updated.GmailTokenExpiry = &newToken.Expiry
		if _, err := s.AuthRepository.UpdateUser(&updated); err != nil {
			return nil, fmt.Errorf("save refreshed Gmail token: %w", err)
		}
		*user = updated
	}
	client := oauth2.NewClient(ctx, tokenSource)
	client.Timeout = gmailSyncTimeout

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, errs.InternalServerError("Gagal membuat Gmail client: " + err.Error())
	}

	return srv, nil
}

func (s *GmailServiceImpl) SyncTradeConfirmations(userID uint) (*dto.GmailSyncResponse, error) {
	return s.SyncTradeConfirmationsContext(context.Background(), userID)
}

// SyncTradeConfirmationsContext allows callers to cancel Gmail and token refresh requests.
func (s *GmailServiceImpl) SyncTradeConfirmationsContext(ctx context.Context, userID uint) (*dto.GmailSyncResponse, error) {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(ctx, gmailSyncTimeout)
	defer cancel()

	// Find user
	user, err := s.AuthRepository.FindByID(userID)
	if err != nil {
		return nil, errs.NotFound("User tidak ditemukan")
	}

	if !user.GmailSyncEnabled {
		return nil, errs.BadRequest("Gmail sync belum diaktifkan. Silakan login ulang via Google.")
	}

	// Create Gmail client
	gmailSrv, err := s.getGmailClient(ctx, user)
	if err != nil {
		return nil, err
	}

	return s.syncTradeConfirmations(ctx, gmailSrv, user, startedAt)
}

func (s *GmailServiceImpl) syncTradeConfirmations(ctx context.Context, gmailSrv *gmail.Service, user *models.Users, startedAt time.Time) (*dto.GmailSyncResponse, error) {
	syncDate := user.LastGmailSyncAt
	if syncDate == nil && !user.CreatedAt.IsZero() {
		syncDate = &user.CreatedAt
	}
	query, err := s.tradeConfirmationQuery(ctx, syncDate)
	if err != nil {
		return nil, err
	}
	var allTrades []dto.ParsedTrade
	var emailCount int
	pageToken := ""
	for {
		messages, err := gmailSrv.Users.Messages.List("me").Q(query).MaxResults(50).PageToken(pageToken).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("search Gmail messages: %w", err)
		}
		for _, msg := range messages.Messages {
			if msg == nil || msg.Id == "" {
				return nil, fmt.Errorf("Gmail search returned a message without an ID")
			}
			fullMsg, err := gmailSrv.Users.Messages.Get("me", msg.Id).Format("full").Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("get Gmail message %s: %w", msg.Id, err)
			}
			trades, err := s.processMessageAttachments(ctx, gmailSrv, fullMsg)
			if err != nil {
				return nil, fmt.Errorf("process Gmail message %s: %w", msg.Id, err)
			}
			allTrades = append(allTrades, trades...)
			emailCount++
		}
		pageToken = messages.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Save parsed trades to database
	var syncedCount int
	if s.TransactionRepo != nil && len(allTrades) > 0 {
		var txReqs []*models.Transaction
		for _, trade := range allTrades {
			var brokerID int
			if s.BrokerRepo != nil && trade.BrokerName != "" {
				broker, err := s.BrokerRepo.GetBrokerByName(user.Id, trade.BrokerName)
				if err == nil && broker != nil {
					brokerID = broker.ID
				}
			}

			amountDone := trade.AmountDone
			if amountDone == 0 {
				amountDone = float64(trade.Lot)*100*trade.Price + trade.Tax
			}

			txReqs = append(txReqs, &models.Transaction{
				UserID:     user.Id,
				Ticker:     trade.Ticker,
				Price:      trade.Price,
				Lot:        trade.Lot,
				BrokerID:   brokerID,
				BrokerName: trade.BrokerName,
				Type:       models.Type(trade.Type),
				Date:       trade.Date,
				AmountDone: amountDone,
				Tax:        trade.Tax,
				RefNo:      trade.RefNo,
			})
		}

		// Sort trades chronologically (oldest date first) to preserve trading sequence
		sort.SliceStable(txReqs, func(i, j int) bool {
			return txReqs[i].Date.Before(txReqs[j].Date)
		})

		synced, err := s.TransactionRepo.SyncTradeConfirmationBatch(user.Id, txReqs)
		if err != nil {
			log.Printf("Failed to sync trade confirmation batch: %v", err)
		}
		syncedCount = synced
	}

	// Save a copy so an unsuccessful repository update cannot advance the
	// caller's checkpoint. Use the start time to retain mail arriving mid-sync.
	updated := *user
	updated.LastGmailSyncAt = &startedAt
	if _, err := s.AuthRepository.UpdateUser(&updated); err != nil {
		return nil, fmt.Errorf("save Gmail sync checkpoint: %w", err)
	}
	*user = updated

	message := fmt.Sprintf("Berhasil parse %d trade dari %d email, disimpan %d trade", len(allTrades), emailCount, syncedCount)
	if s.TransactionRepo == nil {
		message = fmt.Sprintf("Berhasil parse %d trade dari %d email; belum disimpan", len(allTrades), emailCount)
	} else if emailCount == 0 {
		message = "Tidak ada email trade confirmation baru ditemukan"
	}
	return &dto.GmailSyncResponse{
		StatusCode:   http.StatusOK,
		Message:      message,
		TradesFound:  len(allTrades),
		TradesSynced: syncedCount,
		ParsedTrades: allTrades,
	}, nil
}

func (s *GmailServiceImpl) GetSyncStatus(userID uint) (*dto.GmailSyncStatusResponse, error) {
	user, err := s.AuthRepository.FindByID(userID)
	if err != nil {
		return nil, errs.NotFound("User tidak ditemukan")
	}

	return &dto.GmailSyncStatusResponse{
		StatusCode:       http.StatusOK,
		Message:          "Status sync Gmail",
		GmailSyncEnabled: user.GmailSyncEnabled,
		LastSyncAt:       user.LastGmailSyncAt,
	}, nil
}

// buildTradeConfirmationQuery discovers candidate messages; template selectors
// are applied separately when parsing each attachment.
func buildTradeConfirmationQuery(lastSyncAt *time.Time) string {
	parts := []string{
		`subject:("trade confirmation" OR "konfirmasi transaksi" OR "contract note")`,
		"has:attachment",
		"filename:pdf",
	}

	if lastSyncAt != nil {
		// Subtract 1 day to be safe against timezone differences (WITA vs UTC)
		safeDate := lastSyncAt.AddDate(0, 0, -1)
		parts = append(parts, fmt.Sprintf("after:%s", safeDate.Format("2006/01/02")))
	}

	return strings.Join(parts, " ")
}

func (s *GmailServiceImpl) tradeConfirmationQuery(ctx context.Context, lastSyncAt *time.Time) (string, error) {
	query := buildTradeConfirmationQuery(lastSyncAt)
	provider, ok := s.TradeParser.(emailSelectorProvider)
	if !ok {
		return query, nil
	}
	selectors, err := provider.EmailSelectors(ctx)
	if err != nil {
		return "", fmt.Errorf("load Gmail email selectors: %w", err)
	}
	if len(selectors) == 0 {
		return "", fmt.Errorf("no active PDF email selectors")
	}

	var senders []string
	seen := make(map[string]bool)
	for _, selector := range selectors {
		if selector.From == "" {
			// Skip selectors without a sender; they match any sender.
			// Gmail query cannot express this, so rely on local ParseEmail filtering.
			continue
		}
		// Validate even injected providers: only a bare mailbox may enter the
		// quoted Gmail term, never display names, quotes, or query syntax.
		address, err := mail.ParseAddress(selector.From)
		if err != nil || address.Address != selector.From || strings.ContainsAny(selector.From, "\r\n\"\\") {
			return "", fmt.Errorf("invalid Gmail selector sender %q", selector.From)
		}
		key := strings.ToLower(selector.From)
		if !seen[key] {
			senders = append(senders, fmt.Sprintf("from:\"%s\"", selector.From))
			seen[key] = true
		}
	}
	// Add sender filters to query. If no senders configured, rely on subject+attachment query.
	if len(senders) > 0 {
		query += " {" + strings.Join(senders, " ") + "}"
	}
	return query, nil
}

// parseEmailDate attempts to parse an email Date header value across RFC 5322 and common variants.
func parseEmailDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty date")
	}
	if t, err := mail.ParseDate(value); err == nil && !t.IsZero() {
		return t, nil
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil && !t.IsZero() {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse email date: %s", value)
}

func findHeader(part *gmail.MessagePart, name string) string {
	if part == nil {
		return ""
	}
	for _, h := range part.Headers {
		if h != nil && strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	for _, child := range part.Parts {
		if val := findHeader(child, name); val != "" {
			return val
		}
	}
	return ""
}

// extractMessageDate extracts the sent time of the email from the Date header,
// falling back to InternalDate if the header is absent or unparseable.
func extractMessageDate(msg *gmail.Message) time.Time {
	if msg == nil {
		return time.Time{}
	}
	if msg.Payload != nil {
		if dateVal := findHeader(msg.Payload, "Date"); dateVal != "" {
			if t, err := parseEmailDate(dateVal); err == nil && !t.IsZero() {
				return t
			}
		}
	}
	if msg.InternalDate > 0 {
		return time.UnixMilli(msg.InternalDate)
	}
	return time.Time{}
}

// processMessageAttachments downloads and parses PDF attachments from a Gmail message
func (s *GmailServiceImpl) processMessageAttachments(ctx context.Context, gmailSrv *gmail.Service, msg *gmail.Message) ([]dto.ParsedTrade, error) {
	if msg == nil || msg.Payload == nil {
		return nil, fmt.Errorf("Gmail message has no MIME payload")
	}
	emailDate := extractMessageDate(msg)
	meta := parser.EmailMetadata{}
	for _, header := range msg.Payload.Headers {
		if header == nil {
			continue
		}
		switch {
		case strings.EqualFold(header.Name, "From"):
			meta.From = header.Value
		case strings.EqualFold(header.Name, "Subject"):
			meta.Subject = header.Value
		}
	}

	var trades []dto.ParsedTrade
	foundPDF := false
	var visit func(*gmail.MessagePart) error
	visit = func(part *gmail.MessagePart) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if part == nil {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(part.Filename), ".pdf") || strings.EqualFold(part.MimeType, "application/pdf") {
			foundPDF = true
			if part.Body == nil {
				return fmt.Errorf("PDF %q has no body", part.Filename)
			}
			data := part.Body.Data
			if data == "" && part.Body.AttachmentId != "" {
				attachment, err := gmailSrv.Users.Messages.Attachments.Get("me", msg.Id, part.Body.AttachmentId).Context(ctx).Do()
				if err != nil {
					return fmt.Errorf("download PDF %q: %w", part.Filename, err)
				}
				data = attachment.Data
			}
			if data == "" {
				return fmt.Errorf("PDF %q has no data", part.Filename)
			}
			pdfBytes, err := base64.URLEncoding.DecodeString(data)
			if err != nil {
				pdfBytes, err = base64.RawURLEncoding.DecodeString(data)
			}
			if err != nil {
				return fmt.Errorf("decode PDF %q: %w", part.Filename, err)
			}
			if s.TradeParser == nil {
				return fmt.Errorf("trade confirmation parser is not configured")
			}
			var parsed []dto.ParsedTrade
			if emailParser, ok := s.TradeParser.(emailTradeConfirmationParser); ok {
				meta.Filename = part.Filename
				parsed, err = emailParser.ParseEmail(ctx, pdfBytes, meta)
			} else {
				parsed, err = s.TradeParser.Parse(pdfBytes)
			}
			switch {
			case errors.Is(err, parser.ErrEmailNotApplicable) || (err != nil && (strings.Contains(err.Error(), "no extractable text") || strings.Contains(err.Error(), "no active template matches"))):
				// An unrelated or unreadable PDF is not a fatal parse; keep visiting siblings.
				log.Printf("Skipping unreadable or inapplicable PDF %q: %v", part.Filename, err)
			case err != nil:
				return fmt.Errorf("parse PDF %q: %w", part.Filename, err)
			case len(parsed) == 0:
				return fmt.Errorf("parse PDF %q: no trades found", part.Filename)
			default:
				if !emailDate.IsZero() {
					for i := range parsed {
						parsed[i].Date = emailDate
					}
				}
				trades = append(trades, parsed...)
			}
		}
		for _, child := range part.Parts {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(msg.Payload); err != nil {
		return nil, err
	}
	if !foundPDF {
		return nil, fmt.Errorf("Gmail message contains no PDF trades")
	}
	return trades, nil
}
