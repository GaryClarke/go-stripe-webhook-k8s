package config

import (
	"testing"
)

func TestLoad_PORT(t *testing.T) {
	t.Run("default when PORT unset", func(t *testing.T) {
		t.Setenv("PORT", "")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Port != defaultPort {
			t.Fatalf("Port = %q, want %q", cfg.Port, defaultPort)
		}
	})

	t.Run("PORT from env", func(t *testing.T) {
		t.Setenv("PORT", "3000")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Port != "3000" {
			t.Fatalf("Port = %q, want 3000", cfg.Port)
		}
	})

	t.Run("PORT trimmed", func(t *testing.T) {
		t.Setenv("PORT", " 9090 ")
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Port != "9090" {
			t.Fatalf("Port = %q, want 9090", cfg.Port)
		}
	})
}
