package config

import (
	"strings"
	"testing"
)

const testWorkerDatabaseURL = "postgres://webhook:webhook@localhost:5433/stripe_webhook_dev?sslmode=disable"
const testWorkerDownstreamURL = "http://localhost:9999/downstream"

func TestLoadWorker(t *testing.T) {
	cases := []struct {
		name                  string
		brokers               string
		topic                 string
		groupID               string
		databaseURL           string
		noDatabaseURL         bool
		downstreamURL         string
		noDownstreamURL       bool
		wantBrokers           []string
		wantTopic             string
		wantGroupID           string
		wantDatabaseURL       string
		wantDownstreamURL     string
		wantErrSubstr         string
	}{
		{
			name:        "all required env set",
			brokers:     "localhost:19092",
			topic:       "stripe-events",
			groupID:     "stripe-webhook-worker",
			wantBrokers: []string{"localhost:19092"},
			wantTopic:   "stripe-events",
			wantGroupID: "stripe-webhook-worker",
		},
		{
			name:        "brokers comma-separated and trimmed",
			brokers:     " broker1:9092 , broker2:9092 ",
			topic:       "stripe-events",
			groupID:     "my-group",
			wantBrokers: []string{"broker1:9092", "broker2:9092"},
			wantTopic:   "stripe-events",
			wantGroupID: "my-group",
		},
		{
			name:        "topic and group trimmed",
			brokers:     "localhost:19092",
			topic:       "  stripe-events  ",
			groupID:     "  my-group  ",
			wantBrokers: []string{"localhost:19092"},
			wantTopic:   "stripe-events",
			wantGroupID: "my-group",
		},
		{
			name:          "missing KAFKA_BROKERS",
			brokers:       "",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			wantErrSubstr: "KAFKA_BROKERS",
		},
		{
			name:          "blank KAFKA_BROKERS",
			brokers:       "   ",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			wantErrSubstr: "KAFKA_BROKERS",
		},
		{
			name:          "brokers only commas",
			brokers:       ", ,",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			wantErrSubstr: "at least one broker",
		},
		{
			name:          "missing KAFKA_TOPIC",
			brokers:       "localhost:19092",
			topic:         "",
			groupID:       "stripe-webhook-worker",
			wantErrSubstr: "KAFKA_TOPIC",
		},
		{
			name:          "missing KAFKA_GROUP_ID",
			brokers:       "localhost:19092",
			topic:         "stripe-events",
			groupID:       "",
			wantErrSubstr: "KAFKA_GROUP_ID",
		},
		{
			name:          "missing DATABASE_URL",
			brokers:       "localhost:19092",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			noDatabaseURL: true,
			wantErrSubstr: "DATABASE_URL",
		},
		{
			name:          "blank DATABASE_URL",
			brokers:       "localhost:19092",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			databaseURL:   "   ",
			wantErrSubstr: "DATABASE_URL",
		},
		{
			name:          "missing DOWNSTREAM_URL",
			brokers:       "localhost:19092",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			noDownstreamURL: true,
			wantErrSubstr: "DOWNSTREAM_URL",
		},
		{
			name:          "blank DOWNSTREAM_URL",
			brokers:       "localhost:19092",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			downstreamURL: "   ",
			wantErrSubstr: "DOWNSTREAM_URL",
		},
		{
			name:          "invalid DOWNSTREAM_URL scheme",
			brokers:       "localhost:19092",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			downstreamURL: "ftp://example.com/hook",
			wantErrSubstr: "http or https",
		},
		{
			name:          "DOWNSTREAM_URL missing host",
			brokers:       "localhost:19092",
			topic:         "stripe-events",
			groupID:       "stripe-webhook-worker",
			downstreamURL: "http://",
			wantErrSubstr: "host",
		},
		{
			name:              "DOWNSTREAM_URL https trimmed",
			brokers:           "localhost:19092",
			topic:             "stripe-events",
			groupID:           "stripe-webhook-worker",
			downstreamURL:     "  https://mock:8080/downstream  ",
			wantBrokers:       []string{"localhost:19092"},
			wantTopic:         "stripe-events",
			wantGroupID:       "stripe-webhook-worker",
			wantDownstreamURL: "https://mock:8080/downstream",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KAFKA_BROKERS", tc.brokers)
			t.Setenv("KAFKA_TOPIC", tc.topic)
			t.Setenv("KAFKA_GROUP_ID", tc.groupID)
			switch {
			case tc.noDatabaseURL:
				t.Setenv("DATABASE_URL", "")
			case tc.databaseURL != "":
				t.Setenv("DATABASE_URL", tc.databaseURL)
			default:
				t.Setenv("DATABASE_URL", testWorkerDatabaseURL)
			}
			switch {
			case tc.noDownstreamURL:
				t.Setenv("DOWNSTREAM_URL", "")
			case tc.downstreamURL != "":
				t.Setenv("DOWNSTREAM_URL", tc.downstreamURL)
			default:
				t.Setenv("DOWNSTREAM_URL", testWorkerDownstreamURL)
			}

			cfg, err := LoadWorker()
			if tc.wantErrSubstr != "" {
				if err == nil {
					t.Fatal("LoadWorker: err = nil, want error")
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("LoadWorker: err = %q, want substring %q", err.Error(), tc.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadWorker: %v", err)
			}
			if len(cfg.KafkaBrokers) != len(tc.wantBrokers) {
				t.Fatalf("KafkaBrokers = %v, want %v", cfg.KafkaBrokers, tc.wantBrokers)
			}
			for i := range cfg.KafkaBrokers {
				if cfg.KafkaBrokers[i] != tc.wantBrokers[i] {
					t.Fatalf("KafkaBrokers[%d] = %q, want %q", i, cfg.KafkaBrokers[i], tc.wantBrokers[i])
				}
			}
			if cfg.KafkaTopic != tc.wantTopic {
				t.Fatalf("KafkaTopic = %q, want %q", cfg.KafkaTopic, tc.wantTopic)
			}
			if cfg.KafkaGroupID != tc.wantGroupID {
				t.Fatalf("KafkaGroupID = %q, want %q", cfg.KafkaGroupID, tc.wantGroupID)
			}
			wantDatabaseURL := tc.wantDatabaseURL
			if wantDatabaseURL == "" {
				wantDatabaseURL = testWorkerDatabaseURL
			}
			if cfg.DatabaseURL != wantDatabaseURL {
				t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, wantDatabaseURL)
			}
			wantDownstreamURL := tc.wantDownstreamURL
			if wantDownstreamURL == "" {
				wantDownstreamURL = testWorkerDownstreamURL
			}
			if cfg.DownstreamURL != wantDownstreamURL {
				t.Fatalf("DownstreamURL = %q, want %q", cfg.DownstreamURL, wantDownstreamURL)
			}
		})
	}
}
