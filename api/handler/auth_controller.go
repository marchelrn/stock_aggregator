package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"

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
	stateBytes := make([]byte, 16)
	rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	ctx.SetCookie("oauth_state", state, 300, "/", "", false, true)

	url := c.service.Auth.GetGoogleLoginURL(state)
	ctx.Redirect(http.StatusTemporaryRedirect, url)
}

func (c *AuthController) GoogleCallback(ctx *gin.Context) {
	stateCookie, err := ctx.Cookie("oauth_state")
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "State coookie tidak ditemukan"})
		return
	}

	stateQuery := ctx.Query("state")
	if stateCookie != stateQuery {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OAuth state"})
		return
	}

	code := ctx.Query("code")
	if code == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Authorization Code tidak ditemukan"})
		return
	}

	response, err := c.service.Auth.GoogleLogin(code)
	if err != nil {
		ctx.JSON(errs.GetStatusCode(err), gin.H{"error": err.Error()})
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	ctx.SetCookie("oauth_state", "", -1, "/", "", false, true)

	ctx.Redirect(http.StatusTemporaryRedirect, frontendURL+"/auth/callback?token="+response.Token)
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
