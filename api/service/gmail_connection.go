package service

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/dto"
	errs "github.com/marchelrn/stock_api/pkg/error"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const gmailConnectionAudience = "gmail-connection"

func gmailConnectionOAuth(cfg *config.Config) oauth2.Config {
	google := *cfg.GoogleOAuthConfig
	google.Scopes = []string{gmail.GmailReadonlyScope}
	return google
}

func (s *AuthService) StartGmailConnection(userID uint) (*dto.GmailConnectionStart, error) {
	cfg := config.GetConfig()
	verifier := oauth2.GenerateVerifier()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		Audience:  jwt.ClaimStrings{gmailConnectionAudience},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		ID:        oauth2.S256ChallengeFromVerifier(verifier),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(gmailConnectionSigningKey(cfg.JWTSecret))
	if err != nil {
		return nil, errs.InternalServerError("Gagal memulai koneksi Gmail.")
	}
	state := dto.GmailOAuthStatePrefix + signed
	google := gmailConnectionOAuth(cfg)
	return &dto.GmailConnectionStart{
		URL:      google.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.S256ChallengeOption(verifier)),
		State:    state,
		Verifier: verifier,
	}, nil
}

func gmailConnectionSigningKey(secret string) []byte {
	return []byte(gmailConnectionAudience + ":" + secret)
}

func validGmailConnectionState(secret string, userID uint, state, verifier string) bool {
	signed, ok := strings.CutPrefix(state, dto.GmailOAuthStatePrefix)
	if !ok {
		return false
	}
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(signed, claims, func(_ *jwt.Token) (any, error) {
		return gmailConnectionSigningKey(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(), jwt.WithAudience(gmailConnectionAudience),
		jwt.WithSubject(strconv.FormatUint(uint64(userID), 10)))
	return err == nil && claims.ID == oauth2.S256ChallengeFromVerifier(verifier)
}

func (s *AuthService) CompleteGmailConnection(ctx context.Context, userID uint, request dto.GmailConnectionRequest) error {
	cfg := config.GetConfig()
	if !validGmailConnectionState(cfg.JWTSecret, userID, request.State, request.Verifier) {
		return errs.BadRequest("Sesi koneksi Gmail tidak valid atau kedaluwarsa. Coba hubungkan kembali.")
	}

	user, err := s.AuthRepository.FindByID(userID)
	if err != nil {
		return errs.NotFound("User tidak ditemukan")
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	google := gmailConnectionOAuth(cfg)
	token, err := google.Exchange(ctx, request.Code, oauth2.VerifierOption(request.Verifier))
	if err != nil {
		return errs.BadRequest("Kode izin Gmail tidak dapat digunakan. Coba hubungkan kembali.")
	}
	scope, _ := token.Extra("scope").(string)
	if !slices.Contains(strings.Fields(scope), gmail.GmailReadonlyScope) {
		return errs.BadRequest("Izin baca Gmail belum diberikan.")
	}
	if token.RefreshToken == "" {
		return errs.BadRequest("Google belum memberikan akses offline untuk sinkronisasi Gmail. Coba hubungkan kembali.")
	}

	client, err := gmail.NewService(ctx, option.WithHTTPClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))))
	if err != nil {
		return errs.InternalServerError("Gagal menyiapkan koneksi Gmail.")
	}
	profile, err := client.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return errs.BadRequest("Akses Gmail belum dapat diverifikasi. Coba hubungkan kembali.")
	}
	if !strings.EqualFold(profile.EmailAddress, user.Email) {
		return errs.BadRequest("Pilih akun Gmail dengan alamat email yang sama dengan akun Stock Aggregator Anda.")
	}

	access, err := utils.Encrypt(token.AccessToken, cfg.EncryptionKey)
	if err != nil {
		return errs.InternalServerError("Gagal mengamankan token Gmail.")
	}
	refresh, err := utils.Encrypt(token.RefreshToken, cfg.EncryptionKey)
	if err != nil {
		return errs.InternalServerError("Gagal mengamankan token Gmail.")
	}
	user.GmailAccessToken = &access
	user.GmailRefreshToken = &refresh
	user.GmailTokenExpiry = &token.Expiry
	user.GmailSyncEnabled = true
	if _, err := s.AuthRepository.UpdateUser(user); err != nil {
		return errs.InternalServerError("Gagal menyimpan koneksi Gmail.")
	}
	return nil
}
