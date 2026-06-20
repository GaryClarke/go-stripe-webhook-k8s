package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	const (
		dummySecret     = "whsec_test_dummy"
		dummyDatabase   = "postgres://unused"
		trimmedDatabase = "postgres://trimmed"
	)

	cases := []struct {
		name            string
		port            string
		stripeSecret    string
		databaseURL     string
		wantPort        string
		wantSecret      string
		wantDatabaseURL string
		wantErrSubstr   string // non-empty => expect error containing this substring
	}{
		{
			name:            "defaults port when PORT empty",
			port:            "",
			stripeSecret:    dummySecret,
			databaseURL:     dummyDatabase,
			wantPort:        defaultPort,
			wantSecret:      dummySecret,
			wantDatabaseURL: dummyDatabase,
		},
		{
			name:            "PORT from env",
			port:            "3000",
			stripeSecret:    dummySecret,
			databaseURL:     dummyDatabase,
			wantPort:        "3000",
			wantSecret:      dummySecret,
			wantDatabaseURL: dummyDatabase,
		},
		{
			name:            "PORT trimmed",
			port:            " 9090 ",
			stripeSecret:    dummySecret,
			databaseURL:     dummyDatabase,
			wantPort:        "9090",
			wantSecret:      dummySecret,
			wantDatabaseURL: dummyDatabase,
		},
		{
			name:            "STRIPE_WEBHOOK_SECRET trimmed",
			port:            "",
			stripeSecret:    "  " + dummySecret + "  ",
			databaseURL:     dummyDatabase,
			wantPort:        defaultPort,
			wantSecret:      dummySecret,
			wantDatabaseURL: dummyDatabase,
		},
		{
			name:            "DATABASE_URL trimmed",
			port:            "",
			stripeSecret:    dummySecret,
			databaseURL:     "  " + trimmedDatabase + "  ",
			wantPort:        defaultPort,
			wantSecret:      dummySecret,
			wantDatabaseURL: trimmedDatabase,
		},
		{
			name:          "missing STRIPE_WEBHOOK_SECRET",
			port:          "",
			stripeSecret:  "",
			databaseURL:   dummyDatabase,
			wantErrSubstr: "STRIPE_WEBHOOK_SECRET",
		},
		{
			name:          "blank STRIPE_WEBHOOK_SECRET",
			port:          "8080",
			stripeSecret:  "   ",
			databaseURL:   dummyDatabase,
			wantErrSubstr: "STRIPE_WEBHOOK_SECRET",
		},
		{
			name:          "missing DATABASE_URL",
			port:          "",
			stripeSecret:  dummySecret,
			databaseURL:   "",
			wantErrSubstr: "DATABASE_URL",
		},
		{
			name:          "blank DATABASE_URL",
			port:          "",
			stripeSecret:  dummySecret,
			databaseURL:   "   ",
			wantErrSubstr: "DATABASE_URL",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PORT", tc.port)
			t.Setenv("STRIPE_WEBHOOK_SECRET", tc.stripeSecret)
			t.Setenv("DATABASE_URL", tc.databaseURL)

			cfg, err := Load()
			if tc.wantErrSubstr != "" {
				if err == nil {
					t.Fatal("Load: err = nil, want error")
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("Load: err = %q, want substring %q", err.Error(), tc.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Port != tc.wantPort {
				t.Fatalf("Port = %q, want %q", cfg.Port, tc.wantPort)
			}
			if cfg.StripeWebhookSecret != tc.wantSecret {
				t.Fatalf("StripeWebhookSecret = %q, want %q", cfg.StripeWebhookSecret, tc.wantSecret)
			}
			if cfg.DatabaseURL != tc.wantDatabaseURL {
				t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, tc.wantDatabaseURL)
			}
		})
	}
}
