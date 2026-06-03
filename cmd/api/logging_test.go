package main

import (
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
