package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"regexp"
	"strings"

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

	// 1. Check all Jago emails containing RDN
	fmt.Println("=== 1. JAGO EMAILS ABOUT RDN ===")
	resJago, err := srv.Users.Messages.List("me").Q("from:jago RDN").MaxResults(50).Do()
	if err == nil {
		for _, m := range resJago.Messages {
			msg, err := srv.Users.Messages.Get("me", m.Id).Format("full").Do()
			if err != nil {
				continue
			}
			var subj, date string
			for _, h := range msg.Payload.Headers {
				if h.Name == "Subject" {
					subj = h.Value
				}
				if h.Name == "Date" {
					date = h.Value
				}
			}
			body := getBodyText(msg.Payload)
			re := regexp.MustCompile(`<[^>]+>`)
			plain := re.ReplaceAllString(body, " ")
			// Extract nominal if present
			fmt.Printf("[%s] %s\n", date, subj)
			for _, line := range strings.Split(plain, "\n") {
				l := strings.TrimSpace(line)
				if strings.Contains(strings.ToLower(l), "rp") || strings.Contains(strings.ToLower(l), "sebesar") || strings.Contains(strings.ToLower(l), "nominal") {
					fmt.Printf("   -> %s\n", l)
				}
			}
		}
	}

	// 2. Check Statement of Account emails
	fmt.Println("\n=== 2. STATEMENT OF ACCOUNT EMAILS ===")
	resStmt, err := srv.Users.Messages.List("me").Q("from:stockbit subject:\"Statement of Account\"").MaxResults(10).Do()
	if err == nil {
		for _, m := range resStmt.Messages {
			msg, err := srv.Users.Messages.Get("me", m.Id).Format("metadata").MetadataHeaders("Subject", "Date").Do()
			if err != nil {
				continue
			}
			var subj, date string
			for _, h := range msg.Payload.Headers {
				if h.Name == "Subject" {
					subj = h.Value
				}
				if h.Name == "Date" {
					date = h.Value
				}
			}
			fmt.Printf("[%s] %s (ID: %s)\n", date, subj, m.Id)
		}
	}
}

func getBodyText(p *gmail.MessagePart) string {
	if strings.EqualFold(p.MimeType, "text/plain") || strings.EqualFold(p.MimeType, "text/html") {
		b, err := base64.URLEncoding.DecodeString(p.Body.Data)
		if err == nil {
			return string(b)
		}
		b, err = base64.RawURLEncoding.DecodeString(p.Body.Data)
		if err == nil {
			return string(b)
		}
	}
	for _, child := range p.Parts {
		s := getBodyText(child)
		if s != "" {
			return s
		}
	}
	return ""
}
