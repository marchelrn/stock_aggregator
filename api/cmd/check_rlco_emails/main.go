package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/marchelrn/stock_api/config"
	"github.com/marchelrn/stock_api/internal/database"
	"github.com/marchelrn/stock_api/models"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func main() {
	config.Load()
	cfg := config.GetConfig()
	db, sqlDB := database.ConnectDB()
	defer sqlDB.Close()

	var user models.Users
	db.First(&user, 1)

	refreshToken, _ := utils.Decrypt(*user.GmailRefreshToken, cfg.EncryptionKey)
	token := &oauth2.Token{RefreshToken: refreshToken}
	ctx := context.Background()
	ts := cfg.GoogleOAuthConfig.TokenSource(ctx, token)
	client := oauth2.NewClient(ctx, ts)
	gmailSvc, _ := gmail.NewService(ctx, option.WithHTTPClient(client))

	msgIDs := []string{"19b2c16457c2ec33", "19b035caf5cdf8e2"}

	for _, msgID := range msgIDs {
		msg, _ := gmailSvc.Users.Messages.Get("me", msgID).Format("full").Do()

		fmt.Printf("\n========== MSG ID: %s ==========\n", msgID)
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "Date", "Subject", "From", "To", "Message-ID", "Thread-Index", "X-Mailer", "Return-Path", "Received":
				fmt.Printf("  %-15s: %s\n", h.Name, h.Value)
			}
		}
		fmt.Printf("  %-15s: %s\n", "Thread-ID", msg.ThreadId)
		fmt.Printf("  %-15s: %d (labels: %v)\n", "Size", msg.SizeEstimate, msg.LabelIds)

		for _, p := range msg.Payload.Parts {
			if strings.HasSuffix(p.Filename, ".pdf") && p.Body.AttachmentId != "" {
				att, _ := gmailSvc.Users.Messages.Attachments.Get("me", msgID, p.Body.AttachmentId).Do()
				data, _ := base64.URLEncoding.DecodeString(att.Data)
				r, _ := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
				var b bytes.Buffer
				textReader, _ := r.GetPlainText()
				b.ReadFrom(textReader)
				text := b.String()
				// Ambil bagian kecil yang penting
				lines := strings.Split(text, "\n")
				fmt.Printf("\n  File: %s (size: %d bytes)\n", p.Filename, len(data))
				for _, l := range lines {
					l = strings.TrimSpace(l)
					if l != "" && (strings.Contains(l, "Date") || strings.Contains(l, "RLCO") || strings.Contains(l, "REF") || strings.Contains(l, "Allotment") || strings.Contains(l, "Settlement") || strings.Contains(l, "Distribution") || strings.Contains(l, "Refund") || strings.Contains(l, "Buy") || strings.Contains(l, "Total")) {
						fmt.Printf("    %s\n", l)
					}
				}
			}
		}
	}
}
