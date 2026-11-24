package service

import (
	"context"
	"crypto/sha256"
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
	Rights           contract.RightExerciseRepository
	Reviews          contract.TradeReviewRepository
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
const gmailHistoryTimeout = 30 * time.Minute

// Earlier trade markers could include messages discarded by the broker filter.
const tradeConfirmationCacheType = "trade_confirmation:v3"

const cashMutationCacheType = "cash_mutation:v3"

func ImplGmailService(authRepo contract.AuthRepository, parser TradeConfirmationParser, txRepo contract.TransactionRepository, cashRepo contract.CashMutationRepository, brokerRepo contract.BrokerRepository, cacheRepo contract.GmailCacheRepository) *GmailServiceImpl {
	return &GmailServiceImpl{
		AuthRepository:   authRepo,
		TradeParser:      parser,
		TransactionRepo:  txRepo,
		CashMutationRepo: cashRepo,
		BrokerRepo:       brokerRepo,
		GmailCacheRepo:   cacheRepo,
	}
}

func (s *GmailServiceImpl) brokerScope(ctx context.Context, userID uint, selected string) (context.Context, []models.Broker, error) {
	if s.BrokerRepo == nil {
		if selected != "" {
			return ctx, nil, errors.New("broker repository unavailable")
		}
		return ctx, nil, nil
	}
	brokers, err := s.BrokerRepo.GetAllBrokers(userID)
	if err != nil {
		return ctx, nil, err
	}
	names := make(map[string]int)
	var allowed []models.Broker
	for _, b := range brokers {
		if selected != "" && !strings.EqualFold(b.Name, selected) {
			continue
		}
		names[strings.ToLower(strings.TrimSpace(b.Name))] = b.ID
		allowed = append(allowed, b)
	}
	if selected != "" && len(allowed) == 0 {
		return ctx, nil, errs.BadRequest("Broker belum terhubung ke akun ini.")
	}
	return parser.WithBrokerScope(ctx, names), allowed, nil
}

// convertParsedTradeToTransaction converts a parsed trade from email to a transaction model
func (s *GmailServiceImpl) convertParsedTradeToTransaction(user *models.Users, trade *dto.ParsedTrade, brokerID int) *models.Transaction {

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
	date := m.Date.Format("2006-01-02")
	if m.Date.Hour() != 0 || m.Date.Minute() != 0 || m.Date.Second() != 0 || m.Date.Nanosecond() != 0 {
		// Timed entries use the same WITA wall clock as the transactions table.
		date = m.Date.Format("2006-01-02T15:04:05.999999999") + "+08:00"
	}
	return dto.CashMutationItem{
		ID:          int(m.ID),
		UserID:      int(m.UserID),
		BrokerID:    int(m.BrokerID),
		BrokerName:  m.BrokerName,
		Type:        string(m.Type),
		Amount:      m.Amount,
		Date:        date,
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
		return nil, errs.BadRequest("Akun belum terhubung dengan Gmail. Gunakan tombol Hubungkan Gmail.")
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
func (s *GmailServiceImpl) SyncTradeConfirmationsFullContext(ctx context.Context, userID uint, brokerName string) (*dto.GmailSyncResponse, error) {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(ctx, gmailHistoryTimeout)
	defer cancel()

	// Find user
	user, err := s.AuthRepository.FindByID(userID)
	if err != nil {
		return nil, errs.NotFound("User tidak ditemukan")
	}

	if !user.GmailSyncEnabled {
		return nil, errs.BadRequest("Gmail sync belum diaktifkan. Gunakan tombol Hubungkan Gmail.")
	}

	// Create Gmail client
	gmailSrv, err := s.getGmailClient(ctx, user)
	if err != nil {
		return nil, err
	}

	// For full sync, pass nil as syncDate to search all emails from the beginning
	return s.syncTradeConfirmationsWithDate(ctx, gmailSrv, user, startedAt, nil, brokerName)
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
		return nil, errs.BadRequest("Gmail sync belum diaktifkan. Gunakan tombol Hubungkan Gmail.")
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
	return s.syncTradeConfirmationsWithDate(ctx, gmailSrv, user, startedAt, syncDate, "")
}

// syncTradeConfirmationsWithDate performs sync with a specific sync date
func (s *GmailServiceImpl) syncTradeConfirmationsWithDate(ctx context.Context, gmailSrv *gmail.Service, user *models.Users, startedAt time.Time, syncDate *time.Time, selected string) (*dto.GmailSyncResponse, error) {
	ctx, brokers, err := s.brokerScope(ctx, user.Id, selected)
	if err != nil {
		return nil, err
	}
	names := parser.BrokerScope(ctx)
	if names != nil && len(names) == 0 {
		return &dto.GmailSyncResponse{StatusCode: http.StatusOK}, nil
	}
	if len(brokers) > 0 {
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
	var pendingPDFs []dto.PendingTradePDF
	var emailCount int
	var skippedCount, recoveredCount int
	pendingMessages := make(map[string]int)
	knownReviews := make(map[string][]models.TradeReview)
	if s.Reviews != nil {
		rows, err := s.Reviews.List(ctx, user.Id, true)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			knownReviews[row.Digest] = append(knownReviews[row.Digest], row)
		}
	}
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
			reviewed := 0
			trades, unread, err := s.processMessageAttachments(ctx, gmailSrv, fullMsg, user.Id, knownReviews, &reviewed, &recoveredCount)
			if err != nil {
				return nil, fmt.Errorf("process Gmail message %s: %w", msg.Id, err)
			}
			pendingPDFs = append(pendingPDFs, unread...)
			accepted := 0
			for _, trade := range trades {
				if names != nil && names[strings.ToLower(strings.TrimSpace(trade.BrokerName))] == 0 {
					continue
				}
				allTrades = append(allTrades, trade)
				accepted++
			}
			emailCount++

			// A skipped/unreadable PDF is not proof of a completed import.
			if len(unread) == 0 && accepted+reviewed > 0 && accepted == len(trades) {
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

	// Save parsed trades to database
	var syncedCount int
	if s.TransactionRepo != nil && len(allTrades) > 0 {
		txReqs := make([]*models.Transaction, 0, len(allTrades))
		for _, trade := range allTrades {
			transaction := s.convertParsedTradeToTransaction(user, &trade, names[strings.ToLower(strings.TrimSpace(trade.BrokerName))])
			txReqs = append(txReqs, transaction)
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
	if selected == "" && len(pendingPDFs) == 0 {
		updated := *user
		updated.LastGmailSyncAt = &startedAt
		if _, err := s.AuthRepository.UpdateUser(&updated); err != nil {
			return nil, fmt.Errorf("save Gmail sync checkpoint: %w", err)
		}
		*user = updated
	}

	syncedCount += recoveredCount
	message := fmt.Sprintf("Berhasil parse %d trade dari %d email (%d di-skip dari cache), disimpan %d trade", len(allTrades), emailCount, skippedCount, syncedCount)
	if s.TransactionRepo == nil {
		message = fmt.Sprintf("Berhasil parse %d trade dari %d email (%d di-skip); belum disimpan", len(allTrades), emailCount, skippedCount)
	} else if emailCount == 0 && skippedCount == 0 {
		message = "Tidak ada email trade confirmation baru ditemukan"
	} else if emailCount == 0 {
		message = fmt.Sprintf("Semua %d email sudah pernah diproses (dari cache)", skippedCount)
	}
	if len(pendingPDFs) > 0 {
		message = fmt.Sprintf("%d transaksi baru disimpan. %d PDF belum terbaca. %s: %s", syncedCount, len(pendingPDFs), pendingPDFs[0].Filename, pendingPDFs[0].Reason)
		log.Printf("Gmail trade sync: %d pending PDFs; checkpoint retained", len(pendingPDFs))
	}
	return &dto.GmailSyncResponse{
		PendingPDFs:  pendingPDFs,
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
		GmailSyncEnabled: user.GmailSyncEnabled && user.GmailRefreshToken != nil && *user.GmailRefreshToken != "",
		LastSyncAt:       user.LastGmailSyncAt,
	}, nil
}

// SyncCashMutations syncs cash deposit/withdrawal emails from Gmail
func (s *GmailServiceImpl) SyncCashMutations(userID uint) (*dto.GmailCashSyncResponse, error) {
	return s.SyncCashMutationsContext(context.Background(), userID, "")
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
func (s *GmailServiceImpl) SyncCashMutationsContext(ctx context.Context, userID uint, brokerName string) (*dto.GmailCashSyncResponse, error) {
	timeout := gmailSyncTimeout
	if brokerName != "" {
		timeout = gmailHistoryTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Find user
	user, err := s.AuthRepository.FindByID(userID)
	if err != nil {
		return nil, errs.NotFound("User tidak ditemukan")
	}

	if !user.GmailSyncEnabled {
		return nil, errs.BadRequest("Gmail sync belum diaktifkan. Gunakan tombol Hubungkan Gmail.")
	}

	// Create Gmail client
	gmailSrv, err := s.getGmailClient(ctx, user)
	if err != nil {
		return nil, err
	}

	return s.syncBrokerCash(ctx, gmailSrv, user, brokerName)
}

func (s *GmailServiceImpl) syncCashMutations(ctx context.Context, gmailSrv *gmail.Service, user *models.Users) (*dto.GmailCashSyncResponse, error) {
	return s.syncBrokerCash(ctx, gmailSrv, user, "")
}
func (s *GmailServiceImpl) syncBrokerCash(ctx context.Context, gmailSrv *gmail.Service, user *models.Users, selected string) (*dto.GmailCashSyncResponse, error) {
	ctx, brokers, err := s.brokerScope(ctx, user.Id, selected)
	if err != nil {
		return nil, err
	}
	if s.BrokerRepo == nil {
		return s.importBrokerCash(ctx, gmailSrv, user, nil, user.CreatedAt)
	}
	total := &dto.GmailCashSyncResponse{StatusCode: http.StatusOK}
	var failures []error
	for _, broker := range brokers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		scope := parser.WithBrokerScope(ctx, map[string]int{strings.ToLower(strings.TrimSpace(broker.Name)): broker.ID})
		if provider, ok := s.TradeParser.(emailKindSelectorProvider); ok {
			selectors, loadErr := provider.EmailSelectorsForKinds(scope, parser.KindCash, parser.KindDividend)
			if loadErr != nil {
				failures = append(failures, fmt.Errorf("%s: %w", broker.Name, loadErr))
				continue
			}
			if len(selectors) == 0 {
				continue
			}
		}
		since := broker.LastCashSyncAt
		if selected != "" {
			since = nil
		}
		started := time.Now().UTC()
		baseline := user.CreatedAt
		if broker.CashBaseAsOf != nil && broker.CashBaseAsOf.After(baseline) {
			baseline = *broker.CashBaseAsOf
		}
		result, err := s.importBrokerCash(scope, gmailSrv, user, since, baseline)
		if err == nil && s.CashMutationRepo != nil {
			err = s.BrokerRepo.AdvanceCashSync(ctx, user.Id, broker.ID, started)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", broker.Name, err))
			continue
		}
		total.MutationsFound += result.MutationsFound
		total.MutationsSynced += result.MutationsSynced
		total.ParsedMutations = append(total.ParsedMutations, result.ParsedMutations...)
		total.Message = result.Message
	}
	if len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	if len(brokers) != 1 {
		total.Message = fmt.Sprintf("Disimpan %d mutasi kas dari %d broker", total.MutationsSynced, len(brokers))
	}
	return total, nil
}

func (s *GmailServiceImpl) importBrokerCash(ctx context.Context, gmailSrv *gmail.Service, user *models.Users, since *time.Time, baseline time.Time) (*dto.GmailCashSyncResponse, error) {
	registered := parser.BrokerScope(ctx)
	cashParser, err := s.cashParserSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	query, err := s.cashMutationQuery(ctx, since)
	if err != nil {
		return nil, err
	}
	processedIDs := make(map[string]bool)
	if s.GmailCacheRepo != nil {
		cached, err := s.GmailCacheRepo.GetProcessedMessageIDs(ctx, user.Id, cashMutationCacheType)
		if err != nil {
			log.Printf("Warning: failed to load cash Gmail cache: %v", err)
		} else {
			if cached != nil {
				processedIDs = cached
			}
		}
	}
	var allMutations []dto.ParsedCashMutation
	var emailCount int
	var syncedCount int
	var balanceEvents []dto.ParsedCashMutation
	save := func(batch []dto.ParsedCashMutation) error {
		count, err := s.persistCashMutations(user.Id, registered, batch)
		if err != nil {
			return err
		}
		syncedCount += count
		if s.CashMutationRepo != nil && s.GmailCacheRepo != nil && registered != nil {
			for _, m := range batch {
				if err := s.GmailCacheRepo.MarkProcessed(ctx, user.Id, m.MessageID, cashMutationCacheType, 1); err != nil {
					log.Printf("Warning: failed to cache persisted cash message %s: %v", m.MessageID, err)
				}
			}
		}
		return nil
	}
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
		for _, fullMsg := range fullMessages {
			mutations, err := s.processCashMessageWithParser(ctx, gmailSrv, fullMsg, cashParser)
			if err != nil {
				return nil, fmt.Errorf("process Gmail message %s: %w", fullMsg.Id, err)
			}
			for _, mutation := range mutations {
				if registered != nil && registered[strings.ToLower(strings.TrimSpace(mutation.BrokerName))] == 0 {
					continue
				}
				allMutations = append(allMutations, mutation)
				if mutation.EventTime.After(baseline) {
					balanceEvents = append(balanceEvents, mutation)
				} else {
					pageMutations = append(pageMutations, mutation)
				}
				processedIDs[fullMsg.Id] = true
			}
			emailCount++
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := save(pageMutations); err != nil {
			return nil, err
		}
		pageToken = messages.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(balanceEvents, func(i, j int) bool {
		return balanceEvents[i].EventTime.Before(balanceEvents[j].EventTime)
	})
	if err := save(balanceEvents); err != nil {
		return nil, err
	}

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

func (s *GmailServiceImpl) persistCashMutations(userID uint, brokers map[string]int, mutations []dto.ParsedCashMutation) (int, error) {
	if s.CashMutationRepo == nil || len(mutations) == 0 {
		return 0, nil
	}
	requests := make([]*models.CashMutation, 0, len(mutations))
	for _, mutation := range mutations {
		brokerID := brokers[strings.ToLower(strings.TrimSpace(mutation.BrokerName))]
		if brokers != nil && brokerID == 0 {
			return 0, errors.New("broker for cash mutation not found")
		}
		ref := mutation.RefNo
		if ref == "" {
			ref = "gmail:" + mutation.MessageID
		}
		requests = append(requests, &models.CashMutation{
			UserID: userID, BrokerID: brokerID, BrokerName: mutation.BrokerName,
			Type: models.CashMutationType(mutation.Type), Amount: mutation.Amount,
			Date: mutation.EventTime, EmailReceivedAt: mutation.EmailReceivedAt,
			Description: mutation.Description, RefNo: ref,
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

	query, err := selectorQuery(selectors, nil)
	if err != nil {
		return "", err
	}
	if lastSyncAt != nil {
		query += fmt.Sprintf(" after:%d", lastSyncAt.Unix()-1)
	}
	return query, nil
}

func selectorQuery(selectors []parser.EmailMetadata, lastSyncAt *time.Time) (string, error) {
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
					EventTime: parsed.Date, MessageID: msg.Id,
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
		if parser.BrokerScope(ctx) != nil {
			return "", errors.New("broker email selectors unavailable")
		}
		return query, nil
	}
	if err != nil {
		return "", fmt.Errorf("load Gmail email selectors: %w", err)
	}
	if len(selectors) == 0 {
		return "", fmt.Errorf("no active PDF email selectors")
	}

	if parser.BrokerScope(ctx) != nil {
		if s.Rights != nil && parser.BrokerScope(ctx)["stockbit sekuritas"] > 0 {
			for _, subject := range []string{"Statement of Account", "Portfolio Statement", "Laporan Bulanan", "Monthly Statement", "SOA"} {
				selectors = append(selectors, parser.EmailMetadata{FromDomain: "stockbit.com", Subject: subject})
			}
		}
		terms, err := selectorQuery(selectors, lastSyncAt)
		if err != nil {
			return "", err
		}
		return terms + " has:attachment filename:pdf", nil
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
func (s *GmailServiceImpl) processMessageAttachments(ctx context.Context, gmailSrv *gmail.Service, msg *gmail.Message, uid uint, known map[string][]models.TradeReview, reviewed, recoveredCount *int) ([]dto.ParsedTrade, []dto.PendingTradePDF, error) {
	if msg == nil || msg.Payload == nil {
		return nil, nil, fmt.Errorf("Gmail message has no MIME payload")
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
	var pending []dto.PendingTradePDF
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
			pdfBytes, err := readGmailPDF(ctx, gmailSrv, msg.Id, part)
			if err != nil {
				return err
			}
			meta.Filename = part.Filename
			if soaFilename.MatchString(part.Filename) {
				if err := s.recordStockbitSOA(ctx, uid, meta.From, part.Filename, msg.Id, pdfBytes); err != nil {
					pending = append(pending, dto.PendingTradePDF{Filename: part.Filename, Reason: err.Error()})
				} else {
					*reviewed++
				}
				return nil
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(pdfBytes))
			match, canReview := s.TradeParser.(interface {
				ReviewBroker(context.Context, parser.EmailMetadata) (string, error)
			})
			hold := func(row models.TradeReview) {
				if row.State == "saved" {
					*reviewed++
					return
				}
				pending = append(pending, dto.PendingTradePDF{Filename: part.Filename, Reason: "Periksa dokumen di bagian PDF perlu ditinjau."})
			}
			var previous *models.TradeReview
			if len(known[digest]) > 0 && canReview {
				broker, err := match.ReviewBroker(ctx, meta)
				if err != nil {
					return err
				}
				for _, row := range known[digest] {
					if row.BrokerID == parser.BrokerScope(ctx)[strings.ToLower(broker)] {
						if row.State == "pending" {
							previous = &row
							break
						}
						hold(row)
						return nil
					}
				}
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
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if previous != nil && err != nil && (!geminiSyncEnabled(ctx) || errors.Is(err, parser.ErrEmailNotApplicable)) {
				hold(*previous)
				return nil
			}
			if err != nil && ctx.Err() == nil && !errors.Is(err, parser.ErrEmailNotApplicable) && canReview && s.Reviews != nil {
				broker, lookupErr := match.ReviewBroker(ctx, meta)
				if lookupErr != nil {
					return lookupErr
				}
				if id := parser.BrokerScope(ctx)[strings.ToLower(broker)]; id > 0 {
					row, saveErr := s.Reviews.Remember(ctx, &models.TradeReview{UserID: uid, BrokerID: id, BrokerName: broker, MessageID: msg.Id, PartID: part.PartId, Digest: digest, Filename: part.Filename, State: "pending"})
					if saveErr != nil {
						return saveErr
					}
					if row.State == "pending" && geminiSyncEnabled(ctx) {
						n, found, fallbackErr := s.syncGeminiPDF(ctx, uid, row.ID, pdfBytes)
						if fallbackErr == nil {
							row.State = "saved"
							*reviewed += found
							*recoveredCount += n
						} else {
							pending = append(pending, dto.PendingTradePDF{Filename: part.Filename, Reason: "Ekstraksi Gemini belum selesai atau hasil belum lolos validasi. Coba lagi atau tinjau PDF."})
						}
						return nil
					}
					known[digest] = append(known[digest], *row)
					hold(*row)
					return nil
				}
			}
			switch {
			case errors.Is(err, parser.ErrNoStockbitTradeRows):
				pending = append(pending, dto.PendingTradePDF{Filename: part.Filename, Reason: err.Error()})
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
						if parsed[i].BrokerName != "Ajaib Sekuritas" || parsed[i].Date.IsZero() {
							parsed[i].Date = emailDate
						}
					}
				}
				if previous != nil {
					batch := make([]*models.Transaction, 0, len(parsed))
					for _, trade := range parsed {
						if !strings.EqualFold(trade.BrokerName, previous.BrokerName) {
							return errors.New("Broker hasil ekstraksi berbeda dari dokumen.")
						}
						batch = append(batch, s.convertParsedTradeToTransaction(&models.Users{Id: uid}, &trade, previous.BrokerID))
					}
					count, saveErr := s.Reviews.Save(ctx, uid, previous.ID, "pending", batch)
					if saveErr != nil {
						return saveErr
					}
					*reviewed += len(batch)
					*recoveredCount += count
					return nil
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
		return nil, nil, err
	}
	if !foundPDF {
		return nil, nil, fmt.Errorf("Gmail message contains no PDF trades")
	}
	return trades, pending, nil
}
