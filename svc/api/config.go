package api

import (
	"log"
	"os"
	"wappiz/pkg/logger"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	Port               string
	WhatsappBaseURL    string
	WhatsappAPIVersion string
	WebhookVerifyToken string
	WhatsappAppSecret  string
	EncryptionKey      string
	AdminEmail         string
	ResendAPIKey       string
	ResendFromEmail    string
	JWKSEndpoint       string
	JWTIssuer          string // optional; when set the "iss" claim must match
}

func LoadConfiguration() Config {
	if err := godotenv.Load(); err != nil {
		logger.Info("no .env file found, using environment variables")
	}

	return Config{
		DatabaseURL:        mustGet("DATABASE_URL"),
		Port:               getOrDefault("PORT", ":8080"),
		WhatsappBaseURL:    getOrDefault("WHATSAPP_BASE_URL", "https://graph.facebook.com"),
		WhatsappAPIVersion: getOrDefault("WHATSAPP_API_VERSION", "v19.0"),
		WebhookVerifyToken: mustGet("WEBHOOK_VERIFY_TOKEN"),
		WhatsappAppSecret:  mustGet("WHATSAPP_APP_SECRET"),
		EncryptionKey:      mustGet("ENCRYPTION_KEY"),
		AdminEmail:         mustGet("ADMIN_EMAIL"),
		ResendAPIKey:       mustGet("RESEND_API_KEY"),
		ResendFromEmail:    mustGet("RESEND_FROM_EMAIL"),
		JWKSEndpoint:       mustGet("JWKS_ENDPOINT"),
		JWTIssuer:          os.Getenv("JWT_ISSUER"), // optional
	}
}

func mustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("environment variable %s is required", key)
	}
	return v
}

func getOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
