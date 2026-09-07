package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/marchelrn/stock_api/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Config struct {
	Port              string
	IsProd            bool
	DBUrl             string
	JWTSecret         string
	FrontendURL       string
	EncryptionKey     []byte
	GoogleOAuthConfig *oauth2.Config
}

var config *Config

func GetConfig() *Config {
	return config
}

func Load() {
	log.Println("Loading configuration...")

	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 8080
	}

	isProd := utils.SafeCompareString(os.Getenv("ENV"), "production")

	googleOAuthConfig := &oauth2.Config{
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes: []string{
			"email",
			"profile",
			"https://www.googleapis.com/auth/gmail.readonly",
		},
		Endpoint: google.Endpoint,
	}

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if len(encryptionKey) != 32 {
		log.Println("WARNING: ENCRYPTION_KEY must be exactly 32 bytes for AES-256. Using default (NOT safe for production).")
		encryptionKey = "default-encryption-key-32-bytes!"
	}

	config = &Config{
		Port:              strconv.Itoa(port),
		IsProd:            isProd,
		DBUrl:             Production(),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		GoogleOAuthConfig: googleOAuthConfig,
		FrontendURL:       os.Getenv("FRONTEND_URL"),
		EncryptionKey:     []byte(encryptionKey),
	}

}

func Production() string {
	isProd := utils.SafeCompareString(os.Getenv("ENV"), "production")
	if !isProd {
		log.Println("Using local database")
		return LocalDb()
	}
	db_url := os.Getenv("DB_URL")
	if db_url == "" {
		log.Fatal("DB_URL environment variable is not set")
	}
	return db_url
}

func LocalDb() string {
	db_user := os.Getenv("DB_USER")
	db_pass := os.Getenv("DB_PASS")
	db_host := os.Getenv("DB_HOST")
	db_port := os.Getenv("DB_PORT")
	db_name := os.Getenv("DB_NAME")

	return "postgres://" + db_user + ":" + db_pass + "@" + db_host + ":" + db_port + "/" + db_name
}
