package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// TestNewJSONLogger_emitsJSON verifies the Milestone 6 logging contract: one JSON object
// per line on the writer we pass in (production uses os.Stdout; tests use a buffer).
// Central systems (ELK, Datadog, CloudWatch) index these fields later; the app only
// needs a stable JSON shape from slog.NewJSONHandler.
func TestNewJSONLogger_emitsJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)
	logger.Info("test_message", "request_id", "req-test-1", "event_id", "evt_test")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected one log line, got empty buffer")
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, line)
	}

	// slog JSONHandler uses these keys by default (see log/slog JSONHandler docs).
	if fields["msg"] != "test_message" {
		t.Fatalf("msg = %v, want test_message", fields["msg"])
	}
	if fields["level"] != "INFO" {
		t.Fatalf("level = %v, want INFO", fields["level"])
	}
	if fields["request_id"] != "req-test-1" {
		t.Fatalf("request_id = %v, want req-test-1", fields["request_id"])
	}
	if fields["event_id"] != "evt_test" {
		t.Fatalf("event_id = %v, want evt_test", fields["event_id"])
	}
}

// TestNewJSONLogger_nilOptsDefaultsToInfo ensures NewJSONLogger(nil opts) matches main:
// Info-level logs are emitted; Debug is dropped so probe-style noise stays out unless
// we raise the level explicitly in tests or via config later.
func TestNewJSONLogger_nilOptsDefaultsToInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)

	logger.Debug("should_not_appear")
	logger.Info("should_appear")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (Debug filtered); buf=%q", len(lines), buf.String())
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fields["msg"] != "should_appear" {
		t.Fatalf("msg = %v, want should_appear", fields["msg"])
	}
}

// TestNewJSONLogger_customOpts passes HandlerOptions through to the JSON handler so
// tests (or future LOG_LEVEL config) can lower the level without changing call sites.
func TestNewJSONLogger_customOpts(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	logger.Debug("debug_ok")

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected debug line when Level is Debug")
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fields["level"] != "DEBUG" {
		t.Fatalf("level = %v, want DEBUG", fields["level"])
	}
}
