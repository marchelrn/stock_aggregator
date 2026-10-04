package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/contract"
	"github.com/marchelrn/stock_api/dto"
	"github.com/marchelrn/stock_api/models"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type AuthService struct {
	AuthRepository contract.AuthRepository
}

func ImplAuthService(repo contract.AuthRepository) contract.AuthService {
	return &AuthService{
		AuthRepository: repo,
	}
}

func (s *AuthService) generateJWT(user *models.Users) (string, error) {
	cfg := config.GetConfig()

	claims := jwt.MapClaims{
		"user_id": user.Id,
		"email":   user.Email,
		"name":    user.Name,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

func (s *AuthService) userToDTO(user *models.Users) dto.UserData {
	return dto.UserData{
		Id:           user.Id,
		Email:        user.Email,
		Name:         user.Name,
		AuthProvider: user.AuthProvider,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		DeletedAt:    user.DeletedAt,
	}
}

func (s *AuthService) Register(payload *dto.RegisterRequest) (*dto.AuthResponse, error) {
	existingUser, err := s.AuthRepository.FindByEmail(payload.Email)
	if err == nil && existingUser != nil {
		return nil, errs.Conflict("Email sudah terdaftar")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.InternalServerError("Terjadi kesalahan internal: " + err.Error())
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errs.InternalServerError("Gagal Memproses password")
	}

	hashedPwd := string(hashedPassword)

	user := &models.Users{
		Email:        payload.Email,
		Name:         payload.Name,
		Password:     &hashedPwd,
		AuthProvider: "local",
	}

	createdUser, err := s.AuthRepository.CreateUser(user)
	if err != nil {
		return nil, errs.InternalServerError("Gagal membuat akun")
	}

	token, err := s.generateJWT(createdUser)
	if err != nil {
		return nil, errs.InternalServerError("Gagal membuat token")
	}

	return &dto.AuthResponse{
		StatusCode: http.StatusCreated,
		Message:    "Registrasi berhasil",
		Token:      token,
		Data:       s.userToDTO(createdUser),
	}, nil
}

func (s *AuthService) Login(payload *dto.LoginRequest) (*dto.AuthResponse, error) {
	user, err := s.AuthRepository.FindByEmail(payload.Email)
	if err != nil {
		return nil, errs.Unauthorized("Email atau password salah")
	}

	if user.Password == nil {
		return nil, errs.Unauthorized("Akun ini terdaftar via Google silakan login dengan google.")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.Password), []byte(payload.Password)); err != nil {
		return nil, errs.Unauthorized("Email atau password salah")
	}

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, errs.InternalServerError("Gagal membuat token")
	}

	return &dto.AuthResponse{
		StatusCode: http.StatusOK,
		Message:    "Login berhasil",
		Token:      token,
		Data:       s.userToDTO(user),
	}, nil
}

func (s *AuthService) GoogleLogin(code string) (*dto.AuthResponse, error) {
	cfg := config.GetConfig()

	token, err := cfg.GoogleOAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, errs.Unauthorized("Gagal menukar kode otorisasi Google")
	}

	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, errs.InternalServerError("Gagal mendapatkan informasi user Google: " + err.Error())
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errs.InternalServerError("Gagal membaca response Google")
	}

	var googleUser struct {
		Id    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	if err := json.Unmarshal(body, &googleUser); err != nil {
		return nil, errs.InternalServerError("Gagal memparsing user Google")
	}

	user, err := s.AuthRepository.FindByEmail(googleUser.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.InternalServerError("Gagal mencari user")
	}

	if errors.Is(err, gorm.ErrRecordNotFound) || user == nil {
		user = &models.Users{
			Email:        googleUser.Email,
			Name:         googleUser.Name,
			AuthProvider: "google",
			ProviderID:   &googleUser.Id,
		}

		user, err = s.AuthRepository.CreateUser(user)
		if err != nil {
			return nil, errs.InternalServerError("Gagal membuat akun Google")
		}
	} else if user.AuthProvider == "local" {
		user.AuthProvider = "google"
		user.ProviderID = &googleUser.Id

		user, err = s.AuthRepository.UpdateUser(user)
		if err != nil {
			return nil, errs.InternalServerError("Gagal memperbarui user")
		}
	}

	// Encrypt and store Gmail tokens
	if token.AccessToken != "" {
		encAccessToken, err := utils.Encrypt(token.AccessToken, cfg.EncryptionKey)
		if err != nil {
			log.Printf("Warning: failed to encrypt access token: %v", err)
		} else {
			user.GmailAccessToken = &encAccessToken
		}
	}

	if token.RefreshToken != "" {
		encRefreshToken, err := utils.Encrypt(token.RefreshToken, cfg.EncryptionKey)
		if err != nil {
			log.Printf("Warning: failed to encrypt refresh token: %v", err)
		} else {
			user.GmailRefreshToken = &encRefreshToken
		}
	}

	if !token.Expiry.IsZero() {
		user.GmailTokenExpiry = &token.Expiry
	}

	user.GmailSyncEnabled = true
	user, err = s.AuthRepository.UpdateUser(user)
	if err != nil {
		log.Printf("Warning: failed to save Gmail tokens: %v", err)
	}

	tokenString, err := s.generateJWT(user)
	if err != nil {
		return nil, errs.InternalServerError("Gagal membuat token")
	}

	return &dto.AuthResponse{
		StatusCode: http.StatusOK,
		Message:    "Login berhasil",
		Token:      tokenString,
		Data:       s.userToDTO(user),
	}, nil
}

func (s *AuthService) GetGoogleLoginURL(state string) string {
	cfg := config.GetConfig()
	return cfg.GoogleOAuthConfig.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	)
}

func (s *AuthService) GetUserInformation(UserID float64) (*dto.UserResponse, error) {
	user, err := s.AuthRepository.FindByIDFloat64(UserID)
	if err != nil {
		return nil, err
	}

	return &dto.UserResponse{
		StatusCode: http.StatusOK,
		Message:    "User retrieved successfully",
		Data:       s.userToDTO(user),
	}, nil
}
