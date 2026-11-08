package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/marchelrn/stock_api/service/parser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Visit the root as well as nested MIME parts; tolerate absent parts/bodies.
func pdfParts(root *gmail.MessagePart) []*gmail.MessagePart {
	if root == nil {
		return nil
	}
	var found []*gmail.MessagePart
	if root.Filename == originalFilename && root.Body != nil {
		found = append(found, root)
	}
	for _, child := range root.Parts {
		found = append(found, pdfParts(child)...)
	}
	return found
}

func decodeURLBase64(data string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(data)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.URLEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("invalid URL-base64 attachment")
	}
	return decoded, nil
}

type attachmentLoader func(context.Context, string, string) (string, error)

type gmailBaseline struct {
	Data     []byte
	Metadata parser.EmailMetadata
}

func matchingPDF(ctx context.Context, root *gmail.MessagePart, messageID string, original []byte, load attachmentLoader) (*gmailBaseline, bool, error) {
	for _, part := range pdfParts(root) {
		if err := ctx.Err(); err != nil {
			return nil, false, fmt.Errorf("Gmail deadline exceeded")
		}
		data := part.Body.Data
		if data == "" && part.Body.AttachmentId != "" {
			var err error
			data, err = load(ctx, messageID, part.Body.AttachmentId)
			if err != nil {
				return nil, false, fmt.Errorf("cannot download Gmail attachment")
			}
		}
		if data == "" {
			continue
		}
		decoded, err := decodeURLBase64(data)
		if err != nil {
			return nil, false, err
		}
		if sha256.Sum256(decoded) == sha256.Sum256(original) {
			meta := parser.EmailMetadata{Filename: part.Filename}
			for _, header := range root.Headers {
				if header == nil {
					continue
				}
				switch {
				case strings.EqualFold(header.Name, "From"):
					meta.From = header.Value
				case strings.EqualFold(header.Name, "Subject"):
					meta.Subject = header.Value
				}
			}
			return &gmailBaseline{Data: decoded, Metadata: meta}, true, nil
		}
	}
	return nil, false, nil
}

func fetchGmailBaseline(o options, original []byte) (*gmailBaseline, error) {
	// This context also bounds token refresh, not just Gmail API requests.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 15 * time.Second})
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("cannot load .env")
	}
	clientID, secret := os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID == "" || secret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required in .env or environment")
	}
	file, err := os.Open(o.tokenFile)
	if err != nil {
		return nil, fmt.Errorf("cannot open Gmail token; run go run ./cmd/test_gmail_sync from api root for existing OAuth setup")
	}
	var token oauth2.Token
	err = json.NewDecoder(file).Decode(&token)
	closeErr := file.Close()
	if err != nil || closeErr != nil || (token.AccessToken == "" && token.RefreshToken == "") {
		return nil, fmt.Errorf("invalid Gmail token; use existing test_gmail_sync OAuth setup")
	}
	config := oauth2.Config{ClientID: clientID, ClientSecret: secret, Endpoint: google.Endpoint, Scopes: []string{gmail.GmailReadonlyScope}}
	client := config.Client(ctx, &token)
	client.Timeout = 15 * time.Second
	service, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("cannot initialize readonly Gmail client")
	}
	load := func(ctx context.Context, messageID, attachmentID string) (string, error) {
		body, err := service.Users.Messages.Attachments.Get("me", messageID, attachmentID).Context(ctx).Do()
		if err != nil || body == nil {
			return "", fmt.Errorf("attachment request failed")
		}
		return body.Data, nil
	}
	inspect := func(id string) (*gmailBaseline, bool, error) {
		if id == "" {
			return nil, false, fmt.Errorf("empty Gmail message ID")
		}
		message, err := service.Users.Messages.Get("me", id).Format("full").Context(ctx).Do()
		if err != nil || message == nil {
			return nil, false, fmt.Errorf("cannot fetch Gmail message (check token authorization)")
		}
		return matchingPDF(ctx, message.Payload, id, original, load)
	}
	if o.messageID != "" {
		data, found, err := inspect(o.messageID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("explicit message has no original-filename PDF matching local SHA256")
		}
		return data, nil
	}
	// Hard bounds prevent broad custom queries or repeated page tokens from
	// causing unbounded mailbox scans. No fallback to a broader query.
	pageToken := ""
	seenPages := map[string]bool{}
	seenMessages := map[string]bool{}
	inspected := 0
	for page := 0; page < 4 && inspected < 20; page++ {
		if seenPages[pageToken] {
			return nil, fmt.Errorf("Gmail returned a repeated page token")
		}
		seenPages[pageToken] = true
		response, err := service.Users.Messages.List("me").Q(o.query).MaxResults(int64(20 - inspected)).PageToken(pageToken).Context(ctx).Do()
		if err != nil || response == nil {
			return nil, fmt.Errorf("Gmail search failed (check token authorization)")
		}
		for _, message := range response.Messages {
			if inspected >= 20 {
				break
			}
			if message == nil || message.Id == "" || seenMessages[message.Id] {
				continue
			}
			seenMessages[message.Id] = true
			inspected++
			data, found, err := inspect(message.Id)
			if err != nil {
				return nil, err
			}
			if found {
				return data, nil
			}
		}
		if response.NextPageToken == "" {
			break
		}
		pageToken = response.NextPageToken
	}
	return nil, fmt.Errorf("no original-filename PDF matching local SHA256 within search limit (20 messages, 4 pages); try --message-id")
}
