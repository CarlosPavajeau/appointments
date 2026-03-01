package config

import (
	"log"
	"os"

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
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	return &Config{
		DatabaseURL:        mustGet("DATABASE_URL"),
		Port:               getOrDefault("PORT", ":8080"),
		WhatsappBaseURL:    getOrDefault("WHATSAPP_BASE_URL", "https://graph.facebook.com"),
		WhatsappAPIVersion: getOrDefault("WHATSAPP_API_VERSION", "v19.0"),
		WebhookVerifyToken: mustGet("WEBHOOK_VERIFY_TOKEN"),
		WhatsappAppSecret:  mustGet("WHATSAPP_APP_SECRET"),
		EncryptionKey:      mustGet("ENCRYPTION_KEY"),
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
