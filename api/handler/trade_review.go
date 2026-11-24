package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"net/http"
	"strconv"
)

func reviewParams(c *gin.Context) (uint, uint, bool) {
	uid, ok := c.Get("user_id")
	n, okID := uid.(float64)
	if !ok || !okID || n < 1 {
		c.JSON(401, gin.H{"error": "Sesi login tidak valid."})
		return 0, 0, false
	}
	if c.Param("id") == "" {
		return uint(n), 0, true
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "Dokumen tidak valid."})
		return 0, 0, false
	}
	return uint(n), uint(id), true
}
func reviewResponse(c *gin.Context, v any, err error) {
	c.Header("Cache-Control", "no-store")
	if err == nil {
		c.JSON(200, v)
		return
	}
	var app *errs.AppError
	if errors.As(err, &app) {
		c.JSON(app.Code, gin.H{"error": app.Message})
		return
	}
	c.JSON(500, gin.H{"error": "Permintaan belum selesai. Coba lagi."})
}
func (h *GmailController) ListReviews(c *gin.Context) {
	uid, _, ok := reviewParams(c)
	if !ok {
		return
	}
	rows, err := h.service.Gmail.ListReviews(c.Request.Context(), uid)
	reviewResponse(c, rows, err)
}
func (h *GmailController) ReviewDraft(c *gin.Context) {
	uid, id, ok := reviewParams(c)
	if !ok {
		return
	}
	draft, err := h.service.Gmail.ReviewDraft(c.Request.Context(), uid, id)
	reviewResponse(c, draft, err)
}
func (h *GmailController) ExtractReview(c *gin.Context) {
	uid, id, ok := reviewParams(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 12<<20)
	var body struct {
		Consent bool   `json:"consent"`
		Source  string `json:"source"`
		Image   []byte `json:"image"`
	}
	if c.ShouldBindJSON(&body) != nil || !body.Consent || (body.Source != "pdf" && body.Source != "image") || (body.Source == "image") != (len(body.Image) > 0) {
		reviewResponse(c, nil, errs.BadRequest("Pilih sumber dan setujui pengirimannya ke Gemini."))
		return
	}
	draft, err := h.service.Gmail.ExtractReview(c.Request.Context(), uid, id, body.Image)
	reviewResponse(c, draft, err)
}
func (h *GmailController) ConfirmReview(c *gin.Context) {
	uid, id, ok := reviewParams(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
	var body struct {
		Confirmed bool           `json:"confirmed"`
		Draft     dto.TradeDraft `json:"draft"`
	}
	if c.ShouldBindJSON(&body) != nil || !body.Confirmed {
		reviewResponse(c, nil, errs.BadRequest("Periksa dan konfirmasi transaksi terlebih dahulu."))
		return
	}
	n, err := h.service.Gmail.ConfirmReview(c.Request.Context(), uid, id, body.Draft)
	reviewResponse(c, gin.H{"saved": n}, err)
}
