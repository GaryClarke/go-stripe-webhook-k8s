package config

import (
	"testing"
	"time"
)

func TestLoadPublisher(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://webhook:webhook@localhost:5433/stripe_webhook_dev?sslmode=disable")
	t.Setenv("KAFKA_BROKERS", "localhost:19092")
	t.Setenv("KAFKA_TOPIC", "stripe-events")

	cfg, err := LoadPublisher()
	if err != nil {
		t.Fatalf("LoadPublisher: %v", err)
	}
	if cfg.DatabaseURL == "" || cfg.KafkaTopic != "stripe-events" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:19092" {
		t.Fatalf("brokers = %v", cfg.KafkaBrokers)
	}
	if cfg.PollInterval != time.Second {
		t.Fatalf("PollInterval = %v, want 1s", cfg.PollInterval)
	}
}
