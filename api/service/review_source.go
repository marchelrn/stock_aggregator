package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"github.com/marchelrn/stock_api/utils"
	"google.golang.org/api/gmail/v1"
)

func readGmailPDF(ctx context.Context, srv *gmail.Service, msg string, part *gmail.MessagePart) ([]byte, error) {
	if part.Body == nil {
		return nil, errors.New("Lampiran tidak memiliki isi.")
	}
	data := part.Body.Data
	if data == "" && part.Body.AttachmentId != "" {
		body, err := srv.Users.Messages.Attachments.Get("me", msg, part.Body.AttachmentId).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		data = body.Data
	}
	if data == "" {
		return nil, errors.New("Lampiran kosong.")
	}
	decoded, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return base64.RawURLEncoding.DecodeString(data)
	}
	return decoded, nil
}

func (s *GmailServiceImpl) ListReviews(ctx context.Context, uid uint) ([]models.TradeReview, error) {
	return s.Reviews.List(ctx, uid, false)
}
func (s *GmailServiceImpl) ReviewDraft(ctx context.Context, uid, id uint) (*dto.TradeDraft, error) {
	row, err := s.Reviews.Get(ctx, uid, id)
	if err != nil {
		return nil, err
	}
	if row.State != "review" {
		return nil, errs.Conflict("Belum ada hasil yang dapat ditinjau.")
	}
	data, err := utils.Decrypt(row.Draft, config.GetConfig().EncryptionKey)
	if err != nil {
		return nil, err
	}
	var d dto.TradeDraft
	err = json.Unmarshal([]byte(data), &d)
	return &d, err
}
func findPart(p *gmail.MessagePart, id string) *gmail.MessagePart {
	if p == nil {
		return nil
	}
	if p.PartId == id {
		return p
	}
	for _, child := range p.Parts {
		if found := findPart(child, id); found != nil {
			return found
		}
	}
	return nil
}
func (s *GmailServiceImpl) ExtractReview(ctx context.Context, uid, id uint, crop []byte) (*dto.TradeDraft, error) {
	row, err := s.Reviews.Get(ctx, uid, id)
	if err != nil {
		return nil, err
	}
	if row.State == "review" {
		return s.ReviewDraft(ctx, uid, id)
	}
	if row.State != "pending" {
		return nil, errs.Conflict("Dokumen sudah selesai ditinjau.")
	}
	key, model, err := geminiConfig()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, reviewExtractionTimeout)
	defer cancel()
	mime := "application/pdf"
	data := crop
	if crop != nil {
		mime = http.DetectContentType(crop)
		if mime != "image/png" && mime != "image/jpeg" {
			return nil, errs.BadRequest("Pilih gambar PNG atau JPEG.")
		}
	} else {
		user, err := s.AuthRepository.FindByID(uid)
		if err != nil {
			return nil, err
		}
		if !user.GmailSyncEnabled {
			return nil, errs.BadRequest("Hubungkan Gmail terlebih dahulu.")
		}
		srv, err := s.getGmailClient(ctx, user)
		if err != nil {
			return nil, err
		}
		msg, err := srv.Users.Messages.Get("me", row.MessageID).Format("full").Context(ctx).Do()
		if err != nil {
			return nil, errs.BadRequest("Email tidak dapat dibuka. Pilih potongan gambar dari PDF.")
		}
		part := findPart(msg.Payload, row.PartID)
		if part == nil || part.Body == nil || part.Body.Size > maxReviewBytes {
			return nil, errs.BadRequest("Pilih potongan gambar dari PDF ini.")
		}
		data, err = readGmailPDF(ctx, srv, row.MessageID, part)
		if err != nil {
			return nil, err
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != row.Digest {
			return nil, errs.Conflict("Lampiran sudah berubah. Sinkronkan kembali.")
		}
	}
	lease := rand.Text()
	if err := s.Reviews.BeginExtraction(ctx, uid, id, mime, lease); err != nil {
		return nil, err
	}
	defer func() {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = s.Reviews.EndExtraction(release, uid, id, lease)
	}()
	draft, err := extractGemini(ctx, geminiClient, key, model, mime, data)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		return nil, err
	}
	encrypted, err := utils.Encrypt(string(encoded), config.GetConfig().EncryptionKey)
	if err != nil {
		return nil, err
	}
	if err := s.Reviews.Draft(ctx, uid, id, lease, encrypted); err != nil {
		return nil, err
	}
	return draft, nil
}
func (s *GmailServiceImpl) ConfirmReview(ctx context.Context, uid, id uint, d dto.TradeDraft) (int, error) {
	row, err := s.Reviews.Get(ctx, uid, id)
	if err != nil {
		return 0, err
	}
	if row.State == "saved" {
		return 0, nil
	}
	if row.State != "review" {
		return 0, errs.Conflict("Baca dan periksa hasil ekstraksi terlebih dahulu.")
	}
	trades, err := validateTradeDraft(d)
	if err != nil {
		return 0, errs.UnprocessableEntity(err.Error())
	}
	return s.Reviews.Save(ctx, uid, id, "review", trades)
}

type geminiSyncKey struct{}

func WithGeminiSync(ctx context.Context, consent bool) context.Context {
	return context.WithValue(ctx, geminiSyncKey{}, consent)
}
func geminiSyncEnabled(ctx context.Context) bool {
	enabled, _ := ctx.Value(geminiSyncKey{}).(bool)
	return enabled
}

func (s *GmailServiceImpl) syncFallbackPDF(parent context.Context, uid, id uint, pdf []byte, broker string, emailSentAt time.Time) (int, int, error) {
	var key, model string
	source := "sync:ollama:application/pdf"
	if doclingEnabled(broker) {
		source = "sync:docling:application/pdf"
	} else if !localPDFEnabled() {
		var err error
		key, model, err = geminiConfig()
		if err != nil {
			return 0, 0, err
		}
		source = "sync:application/pdf"
	}
	ctx, cancel := context.WithTimeout(parent, reviewExtractionTimeout)
	defer cancel()
	lease := rand.Text()
	if err := s.Reviews.BeginExtraction(ctx, uid, id, source, lease); err != nil {
		return 0, 0, fmt.Errorf("Ekstraksi belum dapat dimulai. Coba lagi.")
	}
	defer func() {
		release, done := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer done()
		_ = s.Reviews.EndExtraction(release, uid, id, lease)
	}()
	var trades []*models.Transaction
	var err error
	if doclingEnabled(broker) {
		var draft *dto.TradeDraft
		draft, err = extractDoclingAjaib(ctx, pdf)
		if err == nil {
			trades, err = validateTradeDraft(*draft)
		}
	} else {
		trades, err = extractValidatedWithProvider(ctx, key, model, pdf, localPDFEnabled(), broker)
	}
	if err != nil {
		return 0, 0, err
	}
	if broker == ajaibBroker && !emailSentAt.IsZero() {
		for _, trade := range trades {
			trade.EmailSentAt = &emailSentAt
		}
	}
	count, err := s.Reviews.Save(ctx, uid, id, "pending", trades)
	if err != nil {
		return 0, 0, fmt.Errorf("Transaksi tervalidasi tetapi gagal disimpan.")
	}
	return count, len(trades), nil
}

func extractValidatedPDF(ctx context.Context, key, model string, pdf []byte) ([]*models.Transaction, error) {
	return extractValidatedWithProvider(ctx, key, model, pdf, false, "")
}
func extractValidatedWithProvider(ctx context.Context, key, model string, pdf []byte, local bool, broker string) ([]*models.Transaction, error) {
	var reason string
	for attempt := 0; attempt < 2; attempt++ {
		var draft *dto.TradeDraft
		var err error
		if local {
			draft, err = extractLocalPDF(ctx, pdf, reason, broker)
		} else {
			draft, err = extractGemini(ctx, geminiClient, key, model, "application/pdf", pdf, reason)
		}
		if err != nil {
			return nil, err
		}
		trades, err := validateTradeDraft(*draft)
		if err == nil {
			return trades, nil
		}
		if attempt == 1 || ctx.Err() != nil {
			return nil, err
		}
		reason = "Read the original document again. Correct only using printed evidence. Validation: " + err.Error()
	}
	return nil, errors.New("Hasil ekstraksi belum valid.")
}
