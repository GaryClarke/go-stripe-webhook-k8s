package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// WorkerConfig holds Kafka consumer settings 
type WorkerConfig struct {
	DatabaseURL   string
	KafkaBrokers  []string
	KafkaTopic    string
	KafkaGroupID  string
	DownstreamURL string
}

// LoadWorker reads worker configuration from the environment.
func LoadWorker() (*WorkerConfig, error) {
	_ = godotenv.Load()

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

	groupID := strings.TrimSpace(os.Getenv("KAFKA_GROUP_ID"))
	if groupID == "" {
		return nil, errors.New("config: KAFKA_GROUP_ID is required")
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, errors.New("config: DATABASE_URL is required")
	}
	downstreamURL := strings.TrimSpace(os.Getenv("DOWNSTREAM_URL"))
	if downstreamURL == "" {
		return nil, errors.New("config: DOWNSTREAM_URL is required")
	}
	downstreamURL, err := validateDownstreamURL(downstreamURL)
	if err != nil {
		return nil, err
	}

	return &WorkerConfig{
		DatabaseURL:   databaseURL,
		KafkaBrokers:  brokers,
		KafkaTopic:    topic,
		KafkaGroupID:  groupID,
		DownstreamURL: downstreamURL,
	}, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateDownstreamURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("config: invalid DOWNSTREAM_URL: %w", err)
	}

	switch u.Scheme {
	case "http", "https":
	default:
		return "", errors.New("config: DOWNSTREAM_URL must use http or https")
	}
	if u.Host == "" {
		return "", errors.New("config: DOWNSTREAM_URL must include a host")
	}
	return raw, nil
}
