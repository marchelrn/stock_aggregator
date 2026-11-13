package main

import (
	"context"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/internal/database"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func main() {
	_ = godotenv.Load(".env")
	config.Load()
	db, _ := database.ConnectDB()

	var user models.Users
	if err := db.First(&user, 1).Error; err != nil {
		log.Fatalf("user 1: %v", err)
	}

	cfg := config.GetConfig()
	rt, _ := utils.Decrypt(*user.GmailRefreshToken, cfg.EncryptionKey)
	at, _ := utils.Decrypt(*user.GmailAccessToken, cfg.EncryptionKey)

	ctx := context.Background()
	token := &oauth2.Token{
		AccessToken:  at,
		RefreshToken: rt,
		TokenType:    "Bearer",
	}
	if user.GmailTokenExpiry != nil {
		token.Expiry = *user.GmailTokenExpiry
	}

	tokenSource := cfg.GoogleOAuthConfig.TokenSource(ctx, token)
	client := oauth2.NewClient(ctx, tokenSource)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("gmail srv: %v", err)
	}

	// 1. Search queries related to cash outflow / statements / bank / bonds / mutual funds
	queries := []string{
		"from:jago",
		"from:stockbit statement",
		"from:stockbit rdn",
		"from:stockbit obligasi",
		"from:stockbit bond",
		"from:stockbit reksadana",
		"from:stockbit penarikan",
		"from:stockbit withdrawal",
		"from:stockbit dividen",
		"subject:\"rekening koran\"",
		"subject:\"monthly statement\"",
		"subject:\"laporan bulanan\"",
	}

	for _, q := range queries {
		req := srv.Users.Messages.List("me").Q(q).MaxResults(15)
		res, err := req.Do()
		if err != nil {
			fmt.Printf("Query %q err: %v\n", q, err)
			continue
		}
		if len(res.Messages) == 0 {
			continue
		}
		fmt.Printf("\n=== Query: %s (Found %d) ===\n", q, len(res.Messages))
		for _, m := range res.Messages {
			msg, err := srv.Users.Messages.Get("me", m.Id).Format("metadata").MetadataHeaders("Subject", "Date", "From").Do()
			if err != nil {
				continue
			}
			var subj, from, date string
			for _, h := range msg.Payload.Headers {
				if h.Name == "Subject" {
					subj = h.Value
				}
				if h.Name == "From" {
					from = h.Value
				}
				if h.Name == "Date" {
					date = h.Value
				}
			}
			fmt.Printf("[%s] From: %s | Subj: %s\n", date, from, subj)
		}
	}
}
