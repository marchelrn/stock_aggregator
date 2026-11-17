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
	"sync"
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
	AuthRepository   contract.AuthRepository
	TradeParser      TradeConfirmationParser
	TransactionRepo  contract.TransactionRepository
	CashMutationRepo contract.CashMutationRepository
	BrokerRepo       contract.BrokerRepository
	GmailCacheRepo   contract.GmailCacheRepository
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

type cashParserProvider interface {
	CashParser(context.Context) (*parser.CashEmailParser, error)
}

func (s *GmailServiceImpl) cashParserSnapshot(ctx context.Context) (*parser.CashEmailParser, error) {
	provider, ok := s.TradeParser.(cashParserProvider)
	if !ok {
		return nil, errors.New("metadata cash parser is not configured")
	}
	return provider.CashParser(ctx)
}

// emailKindSelectorProvider is optional. When available, discovery asks for
// selectors per email kind from the database instead of mixing trade with cash.
type emailKindSelectorProvider interface {
	EmailSelectorsForKinds(context.Context, ...string) ([]parser.EmailMetadata, error)
}

// selectorsForKinds prefers the kind-aware provider and falls back to the
// legacy provider for injected parsers.
func (s *GmailServiceImpl) selectorsForKinds(ctx context.Context, kinds ...string) ([]parser.EmailMetadata, bool, error) {
	if provider, ok := s.TradeParser.(emailKindSelectorProvider); ok {
		selectors, err := provider.EmailSelectorsForKinds(ctx, kinds...)
		return selectors, true, err
	}
	if provider, ok := s.TradeParser.(emailSelectorProvider); ok {
		selectors, err := provider.EmailSelectors(ctx)
		return selectors, true, err
	}
	return nil, false, nil
}

const gmailSyncTimeout = 5 * time.Minute

// v2 ignores legacy markers written before persistence or for skipped PDFs.
const tradeConfirmationCacheType = "trade_confirmation:v2"

const cashMutationCacheType = "cash_mutation:v2"

func ImplGmailService(authRepo contract.AuthRepository, parser TradeConfirmationParser, txRepo contract.TransactionRepository, cashRepo contract.CashMutationRepository, brokerRepo contract.BrokerRepository, cacheRepo contract.GmailCacheRepository) contract.GmailService {
	return &GmailServiceImpl{
		AuthRepository:   authRepo,
		TradeParser:      parser,
		TransactionRepo:  txRepo,
		CashMutationRepo: cashRepo,
		BrokerRepo:       brokerRepo,
		GmailCacheRepo:   cacheRepo,
	}
}

// registeredBrokerNames mengembalikan nama broker milik user dalam huruf kecil.
// Flag kedua menandakan filter tersedia; kalau repo gagal dibaca, sync tidak
// boleh diam-diam menyimpan transaksi dengan broker_id 0.
func (s *GmailServiceImpl) registeredBrokerNames(userID uint) (map[string]bool, bool) {
	if s.BrokerRepo == nil {
		return nil, false
	}
	brokers, err := s.BrokerRepo.GetAllBrokers(userID)
	if err != nil {
		log.Printf("Warning: gagal memuat broker user %d: %v", userID, err)
		return nil, false
	}
	names := make(map[string]bool, len(brokers))
	for _, broker := range brokers {
		names[strings.ToLower(strings.TrimSpace(broker.Name))] = true
	}
	return names, true
}

// filterRegisteredTrades membuang hasil parse dari broker yang tidak
// didaftarkan user, supaya riwayat hanya berisi broker miliknya.
func (s *GmailServiceImpl) filterRegisteredTrades(userID uint, trades []dto.ParsedTrade) []dto.ParsedTrade {
	names, ok := s.registeredBrokerNames(userID)
	if !ok {
		return trades
	}
	kept := make([]dto.ParsedTrade, 0, len(trades))
	skipped := 0
	for _, trade := range trades {
		if names[strings.ToLower(strings.TrimSpace(trade.BrokerName))] {
			kept = append(kept, trade)
			continue
		}
		skipped++
	}
	if skipped > 0 {
		log.Printf("Melewati %d transaksi dari broker yang belum didaftarkan user %d", skipped, userID)
	}
	return kept
}

// filterRegisteredMutations melakukan hal yang sama untuk mutasi kas.
func (s *GmailServiceImpl) filterRegisteredMutations(userID uint, mutations []dto.ParsedCashMutation) []dto.ParsedCashMutation {
	names, ok := s.registeredBrokerNames(userID)
	if !ok {
		return mutations
	}
	kept := make([]dto.ParsedCashMutation, 0, len(mutations))
	skipped := 0
	for _, mutation := range mutations {
		if names[strings.ToLower(strings.TrimSpace(mutation.BrokerName))] {
			kept = append(kept, mutation)
			continue
		}
		skipped++
	}
	if skipped > 0 {
		log.Printf("Melewati %d mutasi kas dari broker yang belum didaftarkan user %d", skipped, userID)
	}
	return kept
}

// convertParsedTradeToTransaction converts a parsed trade from email to a transaction model
func (s *GmailServiceImpl) convertParsedTradeToTransaction(user *models.Users, trade *dto.ParsedTrade) *models.Transaction {
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

	return &models.Transaction{
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
	}
}

// convertCashMutationToDTO converts a cash mutation model to DTO format
func (s *GmailServiceImpl) convertCashMutationToDTO(m *models.CashMutation) dto.CashMutationItem {
	return dto.CashMutationItem{
		ID:          int(m.ID),
		UserID:      int(m.UserID),
		BrokerID:    int(m.BrokerID),
		BrokerName:  m.BrokerName,
		Type:        string(m.Type),
		Amount:      m.Amount,
		Date:        m.Date.Format("2006-01-02T15:04:05Z07:00"),
		Description: m.Description,
		RefNo:       m.RefNo,
		CreatedAt:   m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// normalizeStoredTimestamp rebuilds a timestamp read from a
// `timestamp without time zone` column. The value is written using the
// process's local wall clock, but the driver labels it as UTC; without
// normalizing, an expiry 1 hour ahead appears ~timezone-offset hours ahead,
// so the OAuth token is never refreshed and Gmail rejects the stale one.
func normalizeStoredTimestamp(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local)
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
		expiry = normalizeStoredTimestamp(*user.GmailTokenExpiry)
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

// SyncTradeConfirmationsFullContext performs a full sync of all historical trade confirmations
func (s *GmailServiceImpl) SyncTradeConfirmationsFullContext(ctx context.Context, userID uint) (*dto.GmailSyncResponse, error) {
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

	// For full sync, pass nil as syncDate to search all emails from the beginning
	return s.syncTradeConfirmationsWithDate(ctx, gmailSrv, user, startedAt, nil)
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

	// For incremental sync, use LastGmailSyncAt or CreatedAt
	return s.syncTradeConfirmations(ctx, gmailSrv, user, startedAt)
}

// syncTradeConfirmations performs an incremental sync starting from the user's
// checkpoint (LastGmailSyncAt, falling back to CreatedAt). It delegates to
// syncTradeConfirmationsWithDate and exists so tests can inject a fake Gmail
// service directly.
func (s *GmailServiceImpl) syncTradeConfirmations(ctx context.Context, gmailSrv *gmail.Service, user *models.Users, startedAt time.Time) (*dto.GmailSyncResponse, error) {
	syncDate := user.LastGmailSyncAt
	if syncDate == nil && !user.CreatedAt.IsZero() {
		syncDate = &user.CreatedAt
	}
	return s.syncTradeConfirmationsWithDate(ctx, gmailSrv, user, startedAt, syncDate)
}

// syncTradeConfirmationsWithDate performs sync with a specific sync date
func (s *GmailServiceImpl) syncTradeConfirmationsWithDate(ctx context.Context, gmailSrv *gmail.Service, user *models.Users, startedAt time.Time, syncDate *time.Time) (*dto.GmailSyncResponse, error) {
	if s.BrokerRepo != nil {
		brokers, err := s.BrokerRepo.GetAllBrokers(user.Id)
		if err != nil {
			return nil, fmt.Errorf("load broker PDF credentials: %w", err)
		}
		passwords := make(map[string]string)
		for _, broker := range brokers {
			if broker.PDFPasswordEncrypted == "" {
				continue
			}
			password, err := utils.Decrypt(broker.PDFPasswordEncrypted, config.GetConfig().EncryptionKey)
			if err != nil {
				return nil, errors.New("gagal membuka kredensial PDF broker")
			}
			passwords[broker.Name] = password
		}
		ctx = parser.WithPDFPasswords(ctx, passwords)
	}
	query, err := s.tradeConfirmationQuery(ctx, syncDate)
	if err != nil {
		return nil, err
	}
	var allTrades []dto.ParsedTrade
	var emailCount int
	var skippedCount int
	pendingMessages := make(map[string]int)
	pageToken := ""

	// Load processed message IDs in bulk (more efficient than checking one-by-one)
	processedIDs := make(map[string]bool)
	if s.GmailCacheRepo != nil {
		var err error
		processedIDs, err = s.GmailCacheRepo.GetProcessedMessageIDs(ctx, user.Id, tradeConfirmationCacheType)
		if err != nil {
			log.Printf("Warning: failed to load Gmail cache, will process all messages: %v", err)
		}
	}

	for {
		messages, err := gmailSrv.Users.Messages.List("me").Q(query).MaxResults(50).PageToken(pageToken).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("search Gmail messages: %w", err)
		}
		log.Printf("Found %d messages in Gmail search", len(messages.Messages))
		for _, msg := range messages.Messages {
			if msg == nil || msg.Id == "" {
				return nil, fmt.Errorf("Gmail search returned a message without an ID")
			}

			// Skip if already processed (cache hit)
			if processedIDs[msg.Id] {
				skippedCount++
				continue
			}

			fullMsg, err := gmailSrv.Users.Messages.Get("me", msg.Id).Format("full").Context(ctx).Do()
			if err != nil {
				// Hard failure: abort so the checkpoint is not advanced and the
				// message is retried on the next sync instead of being skipped forever.
				return nil, fmt.Errorf("get Gmail message %s: %w", msg.Id, err)
			}
			trades, err := s.processMessageAttachments(ctx, gmailSrv, fullMsg)
			if err != nil {
				// Inapplicable/unreadable PDFs are soft-skipped inside
				// processMessageAttachments; any error reaching here is fatal.
				return nil, fmt.Errorf("process Gmail message %s: %w", msg.Id, err)
			}
			allTrades = append(allTrades, trades...)
			emailCount++

			// A skipped/unreadable PDF is not proof of a completed import.
			if len(trades) > 0 {
				pendingMessages[msg.Id] = len(trades)
			}
		}
		pageToken = messages.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	allTrades = s.filterRegisteredTrades(user.Id, allTrades)

	// Save parsed trades to database
	var syncedCount int
	if s.TransactionRepo != nil && len(allTrades) > 0 {
		txReqs := make([]*models.Transaction, 0, len(allTrades))
		for _, trade := range allTrades {
			txReqs = append(txReqs, s.convertParsedTradeToTransaction(user, &trade))
		}

		// Sort trades chronologically (oldest date first) to preserve trading sequence
		sort.SliceStable(txReqs, func(i, j int) bool {
			return txReqs[i].Date.Before(txReqs[j].Date)
		})

		synced, err := s.TransactionRepo.SyncTradeConfirmationBatch(user.Id, txReqs)
		if err != nil {
			// Do not advance the checkpoint: the failed groups must be retried
			// on the next sync instead of being skipped forever.
			return nil, fmt.Errorf("save parsed trades: %w", err)
		}
		syncedCount = synced
	}

	// Only persisted batches may be skipped later. A parse-only run or failed
	// batch must leave messages eligible for retry.
	if s.TransactionRepo != nil && s.GmailCacheRepo != nil {
		for messageID, tradeCount := range pendingMessages {
			if err := s.GmailCacheRepo.MarkProcessed(ctx, user.Id, messageID, tradeConfirmationCacheType, tradeCount); err != nil {
				log.Printf("Warning: failed to cache persisted message %s: %v", messageID, err)
			}
		}
	}

	// Save a copy so an unsuccessful repository update cannot advance the
	// caller's checkpoint. Use the start time to retain mail arriving mid-sync.
	updated := *user
	updated.LastGmailSyncAt = &startedAt
	if _, err := s.AuthRepository.UpdateUser(&updated); err != nil {
		return nil, fmt.Errorf("save Gmail sync checkpoint: %w", err)
	}
	*user = updated

	message := fmt.Sprintf("Berhasil parse %d trade dari %d email (%d di-skip dari cache), disimpan %d trade", len(allTrades), emailCount, skippedCount, syncedCount)
	if s.TransactionRepo == nil {
		message = fmt.Sprintf("Berhasil parse %d trade dari %d email (%d di-skip); belum disimpan", len(allTrades), emailCount, skippedCount)
	} else if emailCount == 0 && skippedCount == 0 {
		message = "Tidak ada email trade confirmation baru ditemukan"
	} else if emailCount == 0 {
		message = fmt.Sprintf("Semua %d email sudah pernah diproses (dari cache)", skippedCount)
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

// SyncCashMutations syncs cash deposit/withdrawal emails from Gmail
func (s *GmailServiceImpl) SyncCashMutations(userID uint) (*dto.GmailCashSyncResponse, error) {
	return s.SyncCashMutationsContext(context.Background(), userID)
}

// GetCashMutations returns all stored cash mutations for a user
func (s *GmailServiceImpl) GetCashMutations(userID uint) (*dto.CashMutationsListResponse, error) {
	if s.CashMutationRepo == nil {
		return &dto.CashMutationsListResponse{
			StatusCode: 200,
			Message:    "No cash mutation repository available",
			Data:       []dto.CashMutationItem{},
		}, nil
	}
	mutations, err := s.CashMutationRepo.GetAll(userID)
	if err != nil {
		return nil, err
	}
	items := make([]dto.CashMutationItem, 0, len(mutations))
	for _, m := range mutations {
		items = append(items, s.convertCashMutationToDTO(&m))
	}
	return &dto.CashMutationsListResponse{
		StatusCode: 200,
		Message:    "success",
		Data:       items,
	}, nil
}

// SyncCashMutationsContext allows callers to cancel Gmail and token refresh requests.
func (s *GmailServiceImpl) SyncCashMutationsContext(ctx context.Context, userID uint) (*dto.GmailCashSyncResponse, error) {
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

	return s.syncCashMutations(ctx, gmailSrv, user)
}

func (s *GmailServiceImpl) syncCashMutations(ctx context.Context, gmailSrv *gmail.Service, user *models.Users) (*dto.GmailCashSyncResponse, error) {
	if user.CreatedAt.IsZero() {
		return nil, fmt.Errorf("user registration time is required for cash sync")
	}
	cashParser, err := s.cashParserSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	// Cash history is not imported. Keep this independent of the trade
	// checkpoint and use epoch seconds rather than Gmail's day-only cutoff.
	query, err := s.cashMutationQuery(ctx, nil)
	if err != nil {
		return nil, err
	}
	query += fmt.Sprintf(" after:%d", user.CreatedAt.Unix())
	processedIDs := make(map[string]bool)
	if s.GmailCacheRepo != nil {
		cached, err := s.GmailCacheRepo.GetProcessedMessageIDs(ctx, user.Id, cashMutationCacheType)
		if err != nil {
			log.Printf("Warning: failed to load cash Gmail cache: %v", err)
		} else {
			processedIDs = cached
		}
	}
	registered, filterAvailable := s.registeredBrokerNames(user.Id)
	if s.BrokerRepo != nil && !filterAvailable {
		return nil, errors.New("failed to load registered brokers for cash sync")
	}
	var allMutations []dto.ParsedCashMutation
	var emailCount int
	var syncedCount int
	pageToken := ""
	for {
		messages, err := gmailSrv.Users.Messages.List("me").Q(query).MaxResults(50).PageToken(pageToken).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("search Gmail messages: %w", err)
		}
		uncached := make([]*gmail.Message, 0, len(messages.Messages))
		for _, msg := range messages.Messages {
			if msg == nil || msg.Id == "" {
				return nil, errors.New("Gmail search returned a message without an ID")
			}
			if !processedIDs[msg.Id] {
				uncached = append(uncached, msg)
			}
		}
		fullMessages, err := fetchCashMessages(ctx, gmailSrv, uncached)
		if err != nil {
			return nil, err
		}
		var pageMutations []dto.ParsedCashMutation
		pendingMessages := make(map[string]int)
		for _, fullMsg := range fullMessages {
			mutations, err := s.processCashMessageWithParser(ctx, gmailSrv, fullMsg, cashParser)
			if err != nil {
				return nil, fmt.Errorf("process Gmail message %s: %w", fullMsg.Id, err)
			}
			kept := 0
			for _, mutation := range mutations {
				date, err := time.Parse("2006-01-02", mutation.Date)
				if err != nil {
					return nil, fmt.Errorf("invalid cash mutation date %q: %w", mutation.Date, err)
				}
				if date.After(user.CreatedAt) && (!filterAvailable || registered[strings.ToLower(strings.TrimSpace(mutation.BrokerName))]) {
					pageMutations = append(pageMutations, mutation)
					kept++
				}
			}
			if kept > 0 && kept == len(mutations) {
				pendingMessages[fullMsg.Id] = kept
			}
			emailCount++
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count, err := s.persistCashMutations(user.Id, pageMutations)
		if err != nil {
			return nil, err
		}
		syncedCount += count
		allMutations = append(allMutations, pageMutations...)
		if s.CashMutationRepo != nil && s.GmailCacheRepo != nil && filterAvailable {
			for messageID, count := range pendingMessages {
				if err := s.GmailCacheRepo.MarkProcessed(ctx, user.Id, messageID, cashMutationCacheType, count); err != nil {
					log.Printf("Warning: failed to cache persisted cash message %s: %v", messageID, err)
					continue
				}
				processedIDs[messageID] = true
			}
		}
		pageToken = messages.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Do not touch LastGmailSyncAt here: it is the trade-confirmation
	// checkpoint, and advancing it from the cash sync would skip trade
	// emails that arrived since the last trade sync.

	message := fmt.Sprintf("Berhasil parse %d mutasi kas dari %d email, disimpan %d mutasi", len(allMutations), emailCount, syncedCount)
	if s.CashMutationRepo == nil {
		message = fmt.Sprintf("Berhasil parse %d mutasi kas dari %d email; belum disimpan", len(allMutations), emailCount)
	} else if emailCount == 0 {
		message = "Tidak ada email mutasi kas baru ditemukan"
	}
	return &dto.GmailCashSyncResponse{
		StatusCode:      http.StatusOK,
		Message:         message,
		MutationsFound:  len(allMutations),
		MutationsSynced: syncedCount,
		ParsedMutations: allMutations,
	}, nil
}

func (s *GmailServiceImpl) persistCashMutations(userID uint, mutations []dto.ParsedCashMutation) (int, error) {
	if s.CashMutationRepo == nil || len(mutations) == 0 {
		return 0, nil
	}
	requests := make([]*models.CashMutation, 0, len(mutations))
	for _, mutation := range mutations {
		var brokerID int
		if s.BrokerRepo != nil {
			broker, err := s.BrokerRepo.GetBrokerByName(userID, mutation.BrokerName)
			if err != nil {
				return 0, fmt.Errorf("load broker for cash mutation: %w", err)
			}
			if broker == nil {
				return 0, errors.New("broker for cash mutation not found")
			}
			brokerID = broker.ID
		}
		date, err := time.Parse("2006-01-02", mutation.Date)
		if err != nil {
			return 0, fmt.Errorf("invalid cash mutation date %q: %w", mutation.Date, err)
		}
		requests = append(requests, &models.CashMutation{
			UserID: userID, BrokerID: brokerID, BrokerName: mutation.BrokerName,
			Type: models.CashMutationType(mutation.Type), Amount: mutation.Amount,
			Date: date, EmailReceivedAt: mutation.EmailReceivedAt,
			Description: mutation.Description, RefNo: mutation.RefNo,
		})
	}
	count, err := s.CashMutationRepo.SyncCashMutationBatch(userID, requests)
	if err != nil {
		return 0, fmt.Errorf("sync cash mutation batch: %w", err)
	}
	return count, nil
}

func fetchCashMessages(ctx context.Context, gmailSrv *gmail.Service, messages []*gmail.Message) ([]*gmail.Message, error) {
	for _, message := range messages {
		if message == nil || message.Id == "" {
			return nil, errors.New("Gmail search returned a message without an ID")
		}
	}
	full := make([]*gmail.Message, len(messages))
	failures := make([]error, len(messages))
	var workers sync.WaitGroup
	const concurrency = 4
	for worker := 0; worker < concurrency && worker < len(messages); worker++ {
		workers.Add(1)
		go func(start int) {
			defer workers.Done()
			for i := start; i < len(messages); i += concurrency {
				message, err := gmailSrv.Users.Messages.Get("me", messages[i].Id).Format("full").Context(ctx).Do()
				if err != nil {
					failures[i] = fmt.Errorf("get Gmail message %s: %w", messages[i].Id, err)
					return
				}
				full[i] = message
			}
		}(worker)
	}
	workers.Wait()
	for _, err := range failures {
		if err != nil {
			return nil, err
		}
	}
	return full, nil
}

func (s *GmailServiceImpl) cashMutationQuery(ctx context.Context, lastSyncAt *time.Time) (string, error) {
	selectors, available, err := s.selectorsForKinds(ctx, parser.KindCash, parser.KindDividend)
	if err != nil {
		return "", fmt.Errorf("load cash email selectors: %w", err)
	}
	if !available || len(selectors) == 0 {
		return "", errors.New("no active cash email selectors")
	}

	var terms []string
	seen := make(map[string]bool)
	for _, selector := range selectors {
		var parts []string
		if selector.FromDomain != "" {
			if !parser.ValidSenderDomain(selector.FromDomain) {
				return "", errors.New("invalid Gmail selector domain")
			}
			parts = append(parts, fmt.Sprintf("from:\"%s\"", selector.FromDomain))
		}
		if selector.From != "" {
			address, err := mail.ParseAddress(selector.From)
			if err != nil || address.Address != selector.From || strings.ContainsAny(selector.From, "\r\n\"\\") {
				return "", errors.New("invalid Gmail selector sender")
			}
			parts = append(parts, fmt.Sprintf("from:\"%s\"", selector.From))
		}
		if selector.Subject != "" {
			if strings.ContainsAny(selector.Subject, "\r\n\"\\") {
				return "", errors.New("invalid Gmail selector subject")
			}
			parts = append(parts, fmt.Sprintf("subject:\"%s\"", selector.Subject))
		}
		if len(parts) == 0 {
			return "", errors.New("cash discovery requires a sender or subject selector")
		}
		term := "(" + strings.Join(parts, " ") + ")"
		if !seen[term] {
			terms = append(terms, term)
			seen[term] = true
		}
	}
	query := "{" + strings.Join(terms, " ") + "}"
	if lastSyncAt != nil {
		query += " after:" + lastSyncAt.AddDate(0, 0, -1).Format("2006/01/02")
	}
	return query, nil
}

// processCashMessage processes a Gmail message for cash mutation emails (text/HTML body, not PDF)
func (s *GmailServiceImpl) processCashMessage(ctx context.Context, gmailSrv *gmail.Service, msg *gmail.Message) ([]dto.ParsedCashMutation, error) {
	cashParser, err := s.cashParserSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return s.processCashMessageWithParser(ctx, gmailSrv, msg, cashParser)
}

func (s *GmailServiceImpl) processCashMessageWithParser(ctx context.Context, gmailSrv *gmail.Service, msg *gmail.Message, cashParser *parser.CashEmailParser) ([]dto.ParsedCashMutation, error) {
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

	var mutations []dto.ParsedCashMutation
	var missingAmount error
	var visit func(*gmail.MessagePart) error
	visit = func(part *gmail.MessagePart) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if part == nil {
			return nil
		}
		// Process text/plain and text/html parts for cash mutations
		if strings.EqualFold(part.MimeType, "text/plain") || strings.EqualFold(part.MimeType, "text/html") {
			if part.Filename != "" || part.Body == nil {
				return nil
			}
			data := part.Body.Data
			if data == "" && part.Body.AttachmentId != "" {
				attachment, err := gmailSrv.Users.Messages.Attachments.Get("me", msg.Id, part.Body.AttachmentId).Context(ctx).Do()
				if err != nil {
					return fmt.Errorf("download part %q: %w", part.Filename, err)
				}
				data = attachment.Data
			}
			if data == "" {
				return nil
			}
			bodyBytes, err := base64.URLEncoding.DecodeString(data)
			if err != nil {
				bodyBytes, err = base64.RawURLEncoding.DecodeString(data)
			}
			if err != nil {
				return fmt.Errorf("decode part %q: %w", part.Filename, err)
			}
			body := string(bodyBytes)
			parsed, err := cashParser.Parse(ctx, meta, emailDate, body)
			if errors.Is(err, parser.ErrCashAmountNotFound) {
				missingAmount = err
			} else if err != nil {
				return fmt.Errorf("parse cash email: %w", err)
			}
			if parsed != nil {
				// multipart/alternative usually repeats the same event as HTML
				// and plain text. Never persist it twice or accept conflicting
				// representations of the same message.
				if len(mutations) > 0 {
					prior := mutations[0]
					if prior.BrokerName != parsed.BrokerName || prior.Type != string(parsed.Type) ||
						prior.Amount != parsed.Amount || prior.Date != parsed.Date.Format("2006-01-02") ||
						prior.RefNo != parsed.RefNo || prior.Description != parsed.Description {
						return errors.New("conflicting cash values across email MIME parts")
					}
					return nil
				}
				receivedAt := emailDate
				mutations = append(mutations, dto.ParsedCashMutation{
					BrokerName:      parsed.BrokerName,
					Type:            string(parsed.Type),
					Amount:          parsed.Amount,
					Date:            parsed.Date.Format("2006-01-02"),
					RefNo:           parsed.RefNo,
					Description:     parsed.Description,
					EmailReceivedAt: &receivedAt,
				})
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
	if len(mutations) == 0 && missingAmount != nil {
		return nil, fmt.Errorf("parse cash email: no MIME body contains a readable amount: %w", missingAmount)
	}
	return mutations, nil
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
	selectors, available, err := s.selectorsForKinds(ctx, parser.KindTrade)
	if !available {
		return query, nil
	}
	if err != nil {
		return "", fmt.Errorf("load Gmail email selectors: %w", err)
	}
	if len(selectors) == 0 {
		return "", fmt.Errorf("no active PDF email selectors")
	}

	var senders []string
	var extraSubjects []string
	seenSubjects := map[string]bool{"trade confirmation": true, "konfirmasi transaksi": true, "contract note": true}
	wildcardSender := false
	seen := make(map[string]bool)
	for _, selector := range selectors {
		subject := strings.ToLower(strings.TrimSpace(selector.Subject))
		if strings.ContainsAny(subject, "\r\n\"\\") {
			return "", fmt.Errorf("invalid Gmail selector subject")
		}
		if subject != "" && !seenSubjects[subject] {
			extraSubjects = append(extraSubjects, fmt.Sprintf("\"%s\"", subject))
			seenSubjects[subject] = true
		}
		if selector.From == "" {
			wildcardSender = true
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
	// Discovery must include broker-specific subjects (e.g. Trimegah's "TC").
	// Exact sender/subject/filename matching remains the parser's responsibility.
	if len(extraSubjects) > 0 {
		query = strings.Replace(query, `"contract note")`, `"contract note" OR `+strings.Join(extraSubjects, " OR ")+")", 1)
	}
	if len(senders) > 0 && !wildcardSender {
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
