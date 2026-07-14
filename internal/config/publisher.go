package config

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const defaultPublisherPollInterval = time.Second

// PublisherConfig holds outbox publisher settings (DB + Kafka producer).
type PublisherConfig struct {
	DatabaseURL  string
	KafkaBrokers []string
	KafkaTopic   string
	PollInterval time.Duration
}

// LoadPublisher reads publisher configuration from the environment.
func LoadPublisher() (*PublisherConfig, error) {
	_ = godotenv.Load()

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, errors.New("config: DATABASE_URL is required")
	}

	brokersRaw := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
	if brokersRaw == "" {
		return nil, errors.New("config: KAFKA_BROKERS is required")
	}
	brokers := splitCSV(brokersRaw)
	if len(brokers) == 0 {
		return nil, errors.New("config: KAFKA_BROKERS must contain at least one broker")
	}

	topic := strings.TrimSpace(os.Getenv("KAFKA_TOPIC"))
	if topic == "" {
		return nil, errors.New("config: KAFKA_TOPIC is required")
	}

	poll := defaultPublisherPollInterval
	if raw := strings.TrimSpace(os.Getenv("POLL_INTERVAL")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, errors.New("config: PUBLISHER_POLL_INTERVAL must be a duration (e.g. 1s)")
		}
		if d <= 0 {
			return nil, errors.New("config: PUBLISHER_POLL_INTERVAL must be positive")
		}
		poll = d
	}

	return &PublisherConfig{
		DatabaseURL:  databaseURL,
		KafkaBrokers: brokers,
		KafkaTopic:   topic,
		PollInterval: poll,
	}, nil
}
