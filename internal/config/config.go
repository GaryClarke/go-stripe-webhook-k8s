package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds application configuration from environment variables.
type Config struct {
	StripeWebhookSecret string
}

// Load reads configuration from the environment.
// godotenv.Load is best-effort when a local .env file exists.
func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}, nil
}
