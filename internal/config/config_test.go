package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	const dummySecret = "whsec_test_dummy"

	cases := []struct {
		name          string
		port          string
		stripeSecret  string
		wantPort      string
		wantSecret    string
		wantErrSubstr string // non-empty => expect error containing this substring
	}{
		{
			name:         "defaults port when PORT empty",
			port:         "",
			stripeSecret: dummySecret,
			wantPort:     defaultPort,
			wantSecret:   dummySecret,
		},
		{
			name:         "PORT from env",
			port:         "3000",
			stripeSecret: dummySecret,
			wantPort:     "3000",
			wantSecret:   dummySecret,
		},
		{
			name:         "PORT trimmed",
			port:         " 9090 ",
			stripeSecret: dummySecret,
			wantPort:     "9090",
			wantSecret:   dummySecret,
		},
		{
			name:         "STRIPE_WEBHOOK_SECRET trimmed",
			port:         "",
			stripeSecret: "  " + dummySecret + "  ",
			wantPort:     defaultPort,
			wantSecret:   dummySecret,
		},
		{
			name:          "missing STRIPE_WEBHOOK_SECRET",
			port:          "",
			stripeSecret:  "",
			wantErrSubstr: "STRIPE_WEBHOOK_SECRET",
		},
		{
			name:          "blank STRIPE_WEBHOOK_SECRET",
			port:          "8080",
			stripeSecret:  "   ",
			wantErrSubstr: "STRIPE_WEBHOOK_SECRET",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PORT", tc.port)
			t.Setenv("STRIPE_WEBHOOK_SECRET", tc.stripeSecret)

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
		})
	}
}
