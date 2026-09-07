package main

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
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

func main() {
	// Load .env
	if err := godotenv.Load("../../.env"); err != nil {
		// Try loading from api root
		if err := godotenv.Load(".env"); err != nil {
			log.Println("Warning: .env file not found, using environment variables")
		}
	}

	oauthConfig := &oauth2.Config{
		RedirectURL:  "http://localhost:9090/callback",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes: []string{
			"email",
			"profile",
			"https://www.googleapis.com/auth/gmail.readonly",
		},
		Endpoint: google.Endpoint,
	}

	fmt.Println("=== Gmail Trade Confirmation Test Tool ===")
	fmt.Println()

	// Check if we have a saved token
	token, err := loadToken()
	if err != nil {
		// Need to authenticate
		token = authenticate(oauthConfig)
		saveToken(token)
	} else {
		fmt.Println("✓ Menggunakan token tersimpan")
		// Check if token needs refresh
		if token.Expiry.Before(time.Now()) {
			fmt.Println("↻ Token expired, refreshing...")
			tokenSource := oauthConfig.TokenSource(context.Background(), token)
			newToken, err := tokenSource.Token()
			if err != nil {
				fmt.Println("✗ Gagal refresh token, login ulang...")
				token = authenticate(oauthConfig)
			} else {
				token = newToken
			}
			saveToken(token)
		}
	}

	// Create Gmail client
	client := oauthConfig.Client(context.Background(), token)
	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Gagal membuat Gmail client: %v", err)
	}

	fmt.Println("✓ Gmail client berhasil dibuat")
	fmt.Println()

	// Search for trade confirmation emails
	query := `(subject:"trade confirmation" OR subject:"konfirmasi transaksi" OR subject:"contract note" OR subject:"statement of account" OR subject:"client statement") has:attachment filename:pdf`

	fmt.Printf("🔍 Mencari email dengan query: %s\n", query)
	fmt.Println()

	messages, err := srv.Users.Messages.List("me").Q(query).MaxResults(20).Do()
	if err != nil {
		log.Fatalf("Gagal mencari email: %v", err)
	}

	if len(messages.Messages) == 0 {
		fmt.Println("❌ Tidak ada email trade confirmation ditemukan.")
		fmt.Println()
		fmt.Println("Tips: Pastikan akun Gmail ini menerima email trade confirmation dari sekuritas.")
		fmt.Println("      Coba juga search manual di Gmail dengan query di atas.")

		// Try broader search
		fmt.Println()
		fmt.Println("🔍 Mencoba pencarian lebih luas: has:attachment filename:pdf")
		messages2, err := srv.Users.Messages.List("me").Q("has:attachment filename:pdf").MaxResults(5).Do()
		if err == nil && len(messages2.Messages) > 0 {
			fmt.Printf("   Ditemukan %d email dengan PDF attachment. Berikut subject-nya:\n", len(messages2.Messages))
			for _, msg := range messages2.Messages {
				fullMsg, err := srv.Users.Messages.Get("me", msg.Id).Format("metadata").MetadataHeaders("Subject", "From").Do()
				if err == nil {
					for _, h := range fullMsg.Payload.Headers {
						if h.Name == "Subject" {
							fmt.Printf("   - %s\n", h.Value)
						}
					}
				}
			}
		}
		return
	}

	fmt.Printf("✓ Ditemukan %d email trade confirmation\n", len(messages.Messages))
	fmt.Println()

	// Parse all PDF attachments
	stockbitParser := parser.NewStockbitParser()
	mandiriParser := parser.NewMandiriParser()
	var allTrades []tradeRecord
	var emailsProcessed int

	for _, msg := range messages.Messages {
		fullMsg, err := srv.Users.Messages.Get("me", msg.Id).Do()
		if err != nil {
			log.Printf("⚠ Gagal mengambil email %s: %v", msg.Id, err)
			continue
		}

		// Get subject
		subject := "(no subject)"
		from := ""
		for _, h := range fullMsg.Payload.Headers {
			switch h.Name {
			case "Subject":
				subject = h.Value
			case "From":
				from = h.Value
			}
		}

		fmt.Printf("📧 Processing: %s\n", subject)
		fmt.Printf("   From: %s\n", from)

		if fullMsg.Payload == nil {
			continue
		}

		emailsProcessed++
		pdfFound := false

		// Choose parser based on sender
		var activeParser parser.TradeParser
		if strings.Contains(strings.ToLower(from), "stockbit") {
			activeParser = stockbitParser
		} else {
			activeParser = mandiriParser
		}

		// Search for PDF attachments in all parts (including nested)
		processParts(srv, fullMsg.Payload.Parts, fullMsg.Id, activeParser, &allTrades, subject, &pdfFound)

		if !pdfFound {
			fmt.Println("   ⚠ Tidak ada PDF attachment")
		}
		fmt.Println()
	}

	// Save to CSV
	if len(allTrades) > 0 {
		csvFile := "trade_results.csv"
		saveToCSV(allTrades, csvFile)
		fmt.Printf("✅ Berhasil menyimpan %d trades dari %d email ke %s\n", len(allTrades), emailsProcessed, csvFile)
	} else {
		fmt.Printf("⚠ Tidak ada trade yang berhasil di-parse dari %d email.\n", emailsProcessed)
		fmt.Println("   Kemungkinan format PDF tidak cocok dengan Mandiri Sekuritas parser.")
		fmt.Println("   Coba periksa content PDF secara manual dengan menjalankan:")
		fmt.Println("   go run main.go --dump-pdf")
	}

	// Also save raw email info to CSV for debugging
	saveEmailInfoCSV(srv, messages.Messages)
}

type tradeRecord struct {
	EmailSubject string
	Ticker       string
	Lot          int
	Price        float64
	Type         string
	Date         string
	BrokerName   string
	AmountDone   float64
}

func processParts(srv *gmail.Service, parts []*gmail.MessagePart, msgId string, p parser.TradeParser, trades *[]tradeRecord, subject string, pdfFound *bool) {
	for _, part := range parts {
		// Recursively check nested parts
		if len(part.Parts) > 0 {
			processParts(srv, part.Parts, msgId, p, trades, subject, pdfFound)
		}

		if part.Filename == "" || !strings.HasSuffix(strings.ToLower(part.Filename), ".pdf") {
			continue
		}

		*pdfFound = true
		fmt.Printf("   📎 PDF: %s\n", part.Filename)

		// Download attachment
		attachment, err := srv.Users.Messages.Attachments.Get("me", msgId, part.Body.AttachmentId).Do()
		if err != nil {
			log.Printf("   ⚠ Gagal download attachment: %v", err)
			continue
		}

		// Decode
		pdfBytes, err := base64.URLEncoding.DecodeString(attachment.Data)
		if err != nil {
			log.Printf("   ⚠ Gagal decode attachment: %v", err)
			continue
		}

		fmt.Printf("   📄 PDF size: %d bytes\n", len(pdfBytes))

		// Parse
		parsedTrades, err := p.Parse(pdfBytes)
		if err != nil {
			log.Printf("   ⚠ Gagal parse PDF: %v", err)
			continue
		}

		if len(parsedTrades) == 0 {
			fmt.Println("   ⚠ Tidak ada trade ditemukan dalam PDF ini")
		} else {
			fmt.Printf("   ✓ Ditemukan %d trade(s)\n", len(parsedTrades))
		}

		for _, t := range parsedTrades {
			*trades = append(*trades, tradeRecord{
				EmailSubject: subject,
				Ticker:       t.Ticker,
				Lot:          t.Lot,
				Price:        t.Price,
				Type:         t.Type,
				Date:         t.Date.Format("2006-01-02"),
				BrokerName:   t.BrokerName,
				AmountDone:   t.AmountDone,
			})
			fmt.Printf("      %s %s %d lot @ Rp%.0f = Rp%.0f (%s)\n", t.Type, t.Ticker, t.Lot, t.Price, t.AmountDone, t.Date.Format("2006-01-02"))
		}
	}
}

func saveToCSV(trades []tradeRecord, filename string) {
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Gagal membuat file CSV: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header
	writer.Write([]string{
		"Email Subject", "Ticker", "Type", "Lot", "Price", "Amount Done", "Date", "Broker",
	})

	for _, t := range trades {
		writer.Write([]string{
			t.EmailSubject,
			t.Ticker,
			t.Type,
			fmt.Sprintf("%d", t.Lot),
			fmt.Sprintf("%.2f", t.Price),
			fmt.Sprintf("%.2f", t.AmountDone),
			t.Date,
			t.BrokerName,
		})
	}
}

func saveEmailInfoCSV(srv *gmail.Service, messages []*gmail.Message) {
	file, err := os.Create("email_debug.csv")
	if err != nil {
		log.Printf("Gagal membuat email_debug.csv: %v", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"ID", "Subject", "From", "Date", "Has PDF Attachment", "Attachment Names"})

	for _, msg := range messages {
		fullMsg, err := srv.Users.Messages.Get("me", msg.Id).Format("metadata").MetadataHeaders("Subject", "From", "Date").Do()
		if err != nil {
			continue
		}

		subject, from, date := "", "", ""
		for _, h := range fullMsg.Payload.Headers {
			switch h.Name {
			case "Subject":
				subject = h.Value
			case "From":
				from = h.Value
			case "Date":
				date = h.Value
			}
		}

		// Check for attachments
		hasPDF := "No"
		var attachNames []string
		if fullMsg.Payload != nil {
			for _, part := range fullMsg.Payload.Parts {
				if part.Filename != "" {
					attachNames = append(attachNames, part.Filename)
					if strings.HasSuffix(strings.ToLower(part.Filename), ".pdf") {
						hasPDF = "Yes"
					}
				}
			}
		}

		writer.Write([]string{
			msg.Id, subject, from, date, hasPDF, strings.Join(attachNames, "; "),
		})
	}

	fmt.Println("📋 Email debug info disimpan ke email_debug.csv")
}

// OAuth flow helpers

func authenticate(config *oauth2.Config) *oauth2.Token {
	tokenCh := make(chan *oauth2.Token, 1)

	// Start temporary HTTP server for OAuth callback
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}

		token, err := config.Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, fmt.Sprintf("Token exchange failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html><body style="font-family: sans-serif; text-align: center; margin-top: 100px;">
				<h1>✅ Berhasil Login!</h1>
				<p>Kembali ke terminal untuk melihat hasil.</p>
				<p>Tab ini bisa ditutup.</p>
			</body></html>
		`)

		tokenCh <- token
	})

	server := &http.Server{Addr: ":9090", Handler: mux}
	go server.ListenAndServe()

	// Generate auth URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("🔗 Buka URL berikut di browser untuk login:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Println("⏳ Menunggu login...")

	token := <-tokenCh
	server.Shutdown(context.Background())

	fmt.Println("✓ Login berhasil!")
	return token
}

func saveToken(token *oauth2.Token) {
	f, err := os.Create(".gmail_token.json")
	if err != nil {
		log.Printf("Warning: gagal menyimpan token: %v", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func loadToken() (*oauth2.Token, error) {
	f, err := os.Open(".gmail_token.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	if err := json.NewDecoder(f).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}
