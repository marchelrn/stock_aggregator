package handler

import (
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
)

type AuthController struct {
	service *contract.Service
}

func (c *AuthController) InitService(svc *contract.Service) {
	c.service = svc
}

func (c *AuthController) Register(ctx *gin.Context) {
	var payload dto.RegisterRequest

	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	response, err := c.service.Auth.Register(&payload)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, response)
}

func (c *AuthController) Login(ctx *gin.Context) {
	var payload dto.LoginRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := c.service.Auth.Login(&payload)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *AuthController) GoogleLogin(ctx *gin.Context) {
	state := oauth2.GenerateVerifier()
	verifier := oauth2.GenerateVerifier()
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, gin.H{"state": state, "verifier": verifier, "url": c.service.Auth.GetGoogleLoginURL(state, verifier)})
}

func (c *AuthController) GoogleCallback(ctx *gin.Context) {
	stateQuery := ctx.Query("state")
	if strings.HasPrefix(stateQuery, dto.GmailOAuthStatePrefix) {
		result := url.Values{"state": {stateQuery}}
		if code := ctx.Query("code"); code != "" && ctx.Query("error") == "" {
			result.Set("code", code)
		}
		ctx.Header("Cache-Control", "no-store")
		ctx.Header("Referrer-Policy", "no-referrer")
		ctx.Redirect(http.StatusSeeOther, frontendAuthURL()+"/auth/gmail/callback#"+result.Encode())
		return
	}

	if !loginNonce.MatchString(stateQuery) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Sesi login tidak valid."})
		return
	}
	result := url.Values{"state": {stateQuery}}
	if code := ctx.Query("code"); code != "" && ctx.Query("error") == "" {
		result.Set("code", code)
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Referrer-Policy", "no-referrer")
	ctx.Redirect(http.StatusSeeOther, frontendAuthURL()+"/auth/callback#"+result.Encode())
}

var loginNonce = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func (c *AuthController) CompleteGoogleLogin(ctx *gin.Context) {
	var req struct {
		Code     string `json:"code" binding:"required"`
		Verifier string `json:"verifier" binding:"required"`
	}
	ctx.Header("Cache-Control", "no-store")
	if err := ctx.ShouldBindJSON(&req); err != nil || !loginNonce.MatchString(req.Verifier) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permintaan login tidak valid."})
		return
	}
	response, err := c.service.Auth.GoogleLogin(req.Code, req.Verifier)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, response)
}

func frontendAuthURL() string {
	if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
		return strings.TrimRight(frontendURL, "/")
	}
	return "http://localhost:3000"
}

func (c *AuthController) StartGmailConnection(ctx *gin.Context) {
	id, ok := ctx.MustGet("user_id").(float64)
	if !ok || id <= 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi tidak valid."})
		return
	}
	result, err := c.service.Auth.StartGmailConnection(uint(id))
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, result)
}

func (c *AuthController) CompleteGmailConnection(ctx *gin.Context) {
	id, ok := ctx.MustGet("user_id").(float64)
	if !ok || id <= 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi tidak valid."})
		return
	}
	var request dto.GmailConnectionRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Permintaan koneksi Gmail tidak lengkap."})
		return
	}
	if err := c.service.Auth.CompleteGmailConnection(ctx.Request.Context(), uint(id), request); err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (c *AuthController) GetUserInformation(ctx *gin.Context) {
	userGetID, ok := ctx.Get("user_id")
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "user_id tidak ditemukan"})
		return
	}
	userID := userGetID.(float64)
	user, err := c.service.Auth.GetUserInformation(userID)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(user.StatusCode, user)
}
