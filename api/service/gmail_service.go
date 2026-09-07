package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GmailServiceImpl struct {
	AuthRepository contract.AuthRepository
	TradeParser    TradeConfirmationParser
}

// TradeConfirmationParser is the interface for parsing trade confirmation PDFs
type TradeConfirmationParser interface {
	Parse(pdfBytes []byte) ([]dto.ParsedTrade, error)
	BrokerName() string
}

func ImplGmailService(authRepo contract.AuthRepository, parser TradeConfirmationParser) contract.GmailService {
	return &GmailServiceImpl{
		AuthRepository: authRepo,
		TradeParser:    parser,
	}
}

// getGmailClient creates a Gmail API client using the user's stored OAuth tokens
func (s *GmailServiceImpl) getGmailClient(user *models.Users) (*gmail.Service, error) {
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

	client := cfg.GoogleOAuthConfig.Client(context.Background(), token)

	// If token was refreshed, update in database
	newToken, err := cfg.GoogleOAuthConfig.TokenSource(context.Background(), token).Token()
	if err == nil && newToken.AccessToken != token.AccessToken {
		encAccessToken, encErr := utils.Encrypt(newToken.AccessToken, cfg.EncryptionKey)
		if encErr == nil {
			user.GmailAccessToken = &encAccessToken
			user.GmailTokenExpiry = &newToken.Expiry
			if _, updateErr := s.AuthRepository.UpdateUser(user); updateErr != nil {
				log.Printf("Warning: failed to update refreshed token: %v", updateErr)
			}
		}
	}

	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, errs.InternalServerError("Gagal membuat Gmail client: " + err.Error())
	}

	return srv, nil
}

func (s *GmailServiceImpl) SyncTradeConfirmations(userID uint) (*dto.GmailSyncResponse, error) {
	// Find user
	user, err := s.AuthRepository.FindByID(userID)
	if err != nil {
		return nil, errs.NotFound("User tidak ditemukan")
	}

	if !user.GmailSyncEnabled {
		return nil, errs.BadRequest("Gmail sync belum diaktifkan. Silakan login ulang via Google.")
	}

	// Create Gmail client
	gmailSrv, err := s.getGmailClient(user)
	if err != nil {
		return nil, err
	}

	// Build search query for trade confirmation emails
	// Mandiri Sekuritas typically sends from specific addresses
	query := buildTradeConfirmationQuery(user.LastGmailSyncAt)

	log.Printf("Searching Gmail with query: %s", query)

	// Search for emails
	messages, err := gmailSrv.Users.Messages.List("me").Q(query).MaxResults(50).Do()
	if err != nil {
		return nil, errs.InternalServerError("Gagal mencari email: " + err.Error())
	}

	if len(messages.Messages) == 0 {
		now := time.Now()
		user.LastGmailSyncAt = &now
		s.AuthRepository.UpdateUser(user)

		return &dto.GmailSyncResponse{
			StatusCode:   http.StatusOK,
			Message:      "Tidak ada email trade confirmation baru ditemukan",
			TradesFound:  0,
			TradesSynced: 0,
		}, nil
	}

	var allTrades []dto.ParsedTrade

	for _, msg := range messages.Messages {
		fullMsg, err := gmailSrv.Users.Messages.Get("me", msg.Id).Do()
		if err != nil {
			log.Printf("Warning: gagal mengambil email %s: %v", msg.Id, err)
			continue
		}

		// Look for PDF attachments
		trades := s.processMessageAttachments(gmailSrv, fullMsg)
		allTrades = append(allTrades, trades...)
	}

	// Update last sync timestamp
	now := time.Now()
	user.LastGmailSyncAt = &now
	s.AuthRepository.UpdateUser(user)

	return &dto.GmailSyncResponse{
		StatusCode:   http.StatusOK,
		Message:      fmt.Sprintf("Ditemukan %d trade dari %d email", len(allTrades), len(messages.Messages)),
		TradesFound:  len(allTrades),
		TradesSynced: len(allTrades),
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

// buildTradeConfirmationQuery creates a Gmail search query for Mandiri Sekuritas trade confirmations
func buildTradeConfirmationQuery(lastSyncAt *time.Time) string {
	// Mandiri Sekuritas common sender patterns
	parts := []string{
		"subject:(trade confirmation OR konfirmasi transaksi OR contract note)",
		"has:attachment",
		"filename:pdf",
	}

	if lastSyncAt != nil {
		parts = append(parts, fmt.Sprintf("after:%s", lastSyncAt.Format("2006/01/02")))
	}

	return strings.Join(parts, " ")
}

// processMessageAttachments downloads and parses PDF attachments from a Gmail message
func (s *GmailServiceImpl) processMessageAttachments(gmailSrv *gmail.Service, msg *gmail.Message) []dto.ParsedTrade {
	var trades []dto.ParsedTrade

	if msg.Payload == nil {
		return trades
	}

	// Check all parts for PDF attachments
	for _, part := range msg.Payload.Parts {
		if part.Filename == "" || !strings.HasSuffix(strings.ToLower(part.Filename), ".pdf") {
			continue
		}

		log.Printf("Found PDF attachment: %s in message %s", part.Filename, msg.Id)

		// Download attachment
		attachment, err := gmailSrv.Users.Messages.Attachments.Get("me", msg.Id, part.Body.AttachmentId).Do()
		if err != nil {
			log.Printf("Warning: gagal download attachment %s: %v", part.Filename, err)
			continue
		}

		// Decode base64 attachment data
		pdfBytes, err := base64.URLEncoding.DecodeString(attachment.Data)
		if err != nil {
			log.Printf("Warning: gagal decode attachment %s: %v", part.Filename, err)
			continue
		}

		// Parse the PDF
		parsedTrades, err := s.TradeParser.Parse(pdfBytes)
		if err != nil {
			log.Printf("Warning: gagal parse PDF %s: %v", part.Filename, err)
			continue
		}

		trades = append(trades, parsedTrades...)
	}

	return trades
}
