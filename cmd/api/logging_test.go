package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestResolveRequestID_HonorsHeader(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/webhooks/stripe", nil)
	req.Header.Set("X-Request-ID", "client-req-abc")

	got := resolveRequestID(req)
	if got != "client-req-abc" {
		t.Fatalf("got %q, want client-req-abc", got)
	}
}

func TestResolveRequestID_TrimsWhitespace(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "  trimmed-id  ")

	got := resolveRequestID(req)
	if got != "trimmed-id" {
		t.Fatalf("got %q, want trimmed-id", got)
	}
}

func TestResolveRequestID_GeneratesWhenMissing(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	got := resolveRequestID(req)
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("got %q, want valid UUID: %v", got, err)
	}
}

func TestResolveRequestID_GeneratesWhenWhitespaceOnly(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "   \t  ")

	got := resolveRequestID(req)
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("got %q, want generated UUID: %v", got, err)
	}
}

func TestResolveRequestID_GeneratesWhenTooLong(t *testing.T) {
	t.Parallel()

	longID := strings.Repeat("a", maxRequestIDLength+1)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", longID)

	got := resolveRequestID(req)
	if got == longID {
		t.Fatal("expected generated UUID, got the overlong header value")
	}
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("got %q, want valid UUID: %v", got, err)
	}
}

func TestResolveRequestID_AcceptsMaxLength(t *testing.T) {
	t.Parallel()

	id := strings.Repeat("x", maxRequestIDLength)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", id)

	got := resolveRequestID(req)
	if got != id {
		t.Fatalf("got len %d, want len %d", len(got), len(id))
	}
}

func TestWithRequestContext_storesRequestID(t *testing.T) {
	t.Parallel()

	reqLogger := NewJSONLogger(io.Discard, nil)
	ctx := withRequestContext(context.Background(), "req-ctx-1", reqLogger)

	if got := requestIDFromContext(ctx); got != "req-ctx-1" {
		t.Fatalf("requestIDFromContext = %q, want req-ctx-1", got)
	}
}

func TestLoggerFromContext_usesRequestLoggerWhenSet(t *testing.T) {
	t.Parallel()

	var reqBuf, fallbackBuf bytes.Buffer
	reqLogger := NewJSONLogger(&reqBuf, nil)
	fallback := NewJSONLogger(&fallbackBuf, nil)
	ctx := withRequestContext(context.Background(), "req-1", reqLogger)

	got := loggerFromContext(ctx, fallback)
	if got != reqLogger {
		t.Fatal("expected request-scoped logger from context")
	}
}

func TestLoggerFromContext_usesFallbackWhenMissing(t *testing.T) {
	t.Parallel()

	fallback := NewJSONLogger(io.Discard, nil)

	got := loggerFromContext(context.Background(), fallback)
	if got != fallback {
		t.Fatal("expected fallback logger when context has no request logger")
	}
}

func TestLoggerFromContext_usesFallbackWhenLoggerNil(t *testing.T) {
	t.Parallel()

	fallback := NewJSONLogger(io.Discard, nil)
	ctx := context.WithValue(context.Background(), ctxKeyLogger, (*slog.Logger)(nil))

	got := loggerFromContext(ctx, fallback)
	if got != fallback {
		t.Fatal("expected fallback when stored logger is nil")
	}
}
