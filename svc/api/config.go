package api

import (
	"os"
	"wappiz/pkg/logger"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration values for the API server,
// populated from environment variables (or a .env file).
type Config struct {
	// DatabaseURL is the connection string for the PostgreSQL database (DATABASE_URL).
	DatabaseURL string
	// Port is the address the HTTP server listens on (PORT). Defaults to ":8080".
	Port string
	// WhatsappBaseURL is the base URL for the WhatsApp Cloud API (WHATSAPP_BASE_URL).
	// Defaults to "https://graph.facebook.com".
	WhatsappBaseURL string
	// WhatsappAPIVersion is the WhatsApp Cloud API version to use (WHATSAPP_API_VERSION).
	// Defaults to "v19.0".
	WhatsappAPIVersion string
	// WebhookVerifyToken is the secret token used to verify incoming webhook subscriptions
	// from Meta (WEBHOOK_VERIFY_TOKEN).
	WebhookVerifyToken string
	// WhatsappAppSecret is the app secret used to validate the X-Hub-Signature-256 header
	// on incoming webhook payloads (WHATSAPP_APP_SECRET).
	WhatsappAppSecret string
	// EncryptionKey is the key used to encrypt sensitive data at rest (ENCRYPTION_KEY).
	EncryptionKey string
	// AdminEmail is the email address of the default admin user (ADMIN_EMAIL).
	AdminEmail string
	// ResendAPIKey is the API key for the Resend email delivery service (RESEND_API_KEY).
	ResendAPIKey string
	// ResendFromEmail is the sender address used for outgoing emails (RESEND_FROM_EMAIL).
	ResendFromEmail string
	// JWKSEndpoint is the URL of the JSON Web Key Set used to verify JWT signatures (JWKS_ENDPOINT).
	JWKSEndpoint string
	// JWTIssuer is the expected "iss" claim value for incoming JWTs (JWT_ISSUER).
	// Optional — when empty the issuer claim is not validated.
	JWTIssuer string
}

// LoadConfiguration reads configuration from a .env file if present, then falls back
// to the process environment. Fields without defaults will cause the process to exit if
// their corresponding environment variable is not set.
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

// mustGet returns the value of the environment variable identified by key.
// If the variable is absent or empty the error is logged and the process exits with status 1.
func mustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		logger.Error("missing environment variable: " + key)
		os.Exit(1)
	}
	return v
}

// getOrDefault returns the value of the environment variable identified by key,
// or def when the variable is absent or empty.
func getOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
