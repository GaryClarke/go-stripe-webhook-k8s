package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// WorkerConfig holds Kafka consumer settings 
type WorkerConfig struct {
	DatabaseURL  string
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
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

	return &WorkerConfig{
		DatabaseURL:  databaseURL,
		KafkaBrokers: brokers,
		KafkaTopic:   topic,
		KafkaGroupID: groupID,
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
