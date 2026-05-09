package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// defaultPort is used when env PORT is unset or blank.
const defaultPort = "8080"

// Config holds application configuration from environment variables.
type Config struct {
	StripeWebhookSecret string
	// Port is the listen port without a leading colon (e.g. "8080"). Use ":"+cfg.Port for http.Server.Addr.
	Port string
}

// Load reads configuration from the environment.
// godotenv.Load is best-effort when a local .env file exists.
func Load() (*Config, error) {
	_ = godotenv.Load()

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = defaultPort
	}

	stripeWebhookSecret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	if stripeWebhookSecret == "" {
		return nil, errors.New("config: STRIPE_WEBHOOK_SECRET is required")
	}

	return &Config{
		StripeWebhookSecret: stripeWebhookSecret,
		Port:                port,
	}, nil
}
