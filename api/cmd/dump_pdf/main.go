package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	pdflib "github.com/ledongthuc/pdf"
	"bytes"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		godotenv.Load(".env")
	}

	oauthConfig := &oauth2.Config{
		RedirectURL:  "http://localhost:9090/callback",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{"email", "profile", "https://www.googleapis.com/auth/gmail.readonly"},
		Endpoint:     google.Endpoint,
	}

	token, err := loadToken()
	if err != nil {
		log.Fatal("No saved token. Run test_gmail_sync first to authenticate.")
	}

	client := oauthConfig.Client(context.Background(), token)
	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Failed to create Gmail client: %v", err)
	}

	// Get specific Stockbit Trade Confirmation email
	query := `from:no-reply@stockbit.com subject:"Trade Confirmation" has:attachment filename:pdf`
	messages, err := srv.Users.Messages.List("me").Q(query).MaxResults(3).Do()
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	for _, msg := range messages.Messages {
		fullMsg, err := srv.Users.Messages.Get("me", msg.Id).Do()
		if err != nil {
			continue
		}

		subject := ""
		for _, h := range fullMsg.Payload.Headers {
			if h.Name == "Subject" {
				subject = h.Value
			}
		}

		fmt.Printf("=== %s ===\n", subject)

		if fullMsg.Payload == nil {
			continue
		}

		for _, part := range fullMsg.Payload.Parts {
			if part.Filename == "" || !strings.HasSuffix(strings.ToLower(part.Filename), ".pdf") {
				continue
			}

			attachment, err := srv.Users.Messages.Attachments.Get("me", msg.Id, part.Body.AttachmentId).Do()
			if err != nil {
				continue
			}

			pdfBytes, err := base64.URLEncoding.DecodeString(attachment.Data)
			if err != nil {
				continue
			}

			// Extract text
			reader := bytes.NewReader(pdfBytes)
			pdfReader, err := pdflib.NewReader(reader, int64(len(pdfBytes)))
			if err != nil {
				fmt.Printf("PDF Error: %v\n", err)
				continue
			}

			for i := 1; i <= pdfReader.NumPage(); i++ {
				page := pdfReader.Page(i)
				if page.V.IsNull() {
					continue
				}
				text, err := page.GetPlainText(nil)
				if err != nil {
					continue
				}
				fmt.Printf("--- Page %d ---\n%s\n", i, text)
			}
		}
		fmt.Println()
	}

	// Also dump Stockbit SOA that DID parse
	fmt.Println("\n\n========== STOCKBIT SOA (PARSED OK) ==========")
	query2 := `from:no-reply@stockbit.com subject:"Statement of Account" has:attachment filename:pdf`
	messages2, err := srv.Users.Messages.List("me").Q(query2).MaxResults(1).Do()
	if err == nil {
		for _, msg := range messages2.Messages {
			fullMsg, _ := srv.Users.Messages.Get("me", msg.Id).Do()
			if fullMsg.Payload == nil {
				continue
			}
			for _, part := range fullMsg.Payload.Parts {
				if part.Filename == "" || !strings.HasSuffix(strings.ToLower(part.Filename), ".pdf") {
					continue
				}
				attachment, _ := srv.Users.Messages.Attachments.Get("me", msg.Id, part.Body.AttachmentId).Do()
				pdfBytes, _ := base64.URLEncoding.DecodeString(attachment.Data)
				reader := bytes.NewReader(pdfBytes)
				pdfReader, err := pdflib.NewReader(reader, int64(len(pdfBytes)))
				if err != nil {
					continue
				}
				for i := 1; i <= pdfReader.NumPage(); i++ {
					page := pdfReader.Page(i)
					if page.V.IsNull() {
						continue
					}
					text, _ := page.GetPlainText(nil)
					fmt.Printf("--- SOA Page %d ---\n%s\n", i, text)
				}
			}
		}
	}
}

func loadToken() (*oauth2.Token, error) {
	f, err := os.Open(".gmail_token.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var token oauth2.Token
	json.NewDecoder(f).Decode(&token)
	return &token, nil
}
