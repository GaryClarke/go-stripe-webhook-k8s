package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// Config holds application configuration from environment variables.
type Config struct {
	QueueBackend        string // "memory" or "sqs"
	SQSQueueURL         string
	AWSRegion           string
	StripeWebhookSecret string
}

// Load reads configuration from the environment.
// Call godotenv.Load() first so .env is used in local dev.
// Returns an error if required vars are missing for the chosen backend.
func Load() (*Config, error) {
	_ = godotenv.Load() // ignore error if .env doesn't exist

	cfg := &Config{
		QueueBackend:        os.Getenv("QUEUE_BACKEND"),
		SQSQueueURL:         os.Getenv("SQS_QUEUE_URL"),
		AWSRegion:           os.Getenv("AWS_REGION"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}

	if cfg.QueueBackend == "" {
		cfg.QueueBackend = "memory"
	}

	if cfg.QueueBackend == "sqs" {
		if cfg.SQSQueueURL == "" {
			return nil, errors.New("SQS_QUEUE_URL required when QUEUE_BACKEND=sqs")
		}
		if cfg.AWSRegion == "" {
			return nil, errors.New("AWS_REGION required when QUEUE_BACKEND=sqs")
		}
	}

	return cfg, nil
}
