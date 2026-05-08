package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// apiHandler matches main: panic recovery wraps the mux (same stack as production).
func apiHandler() http.Handler {
	return Recover(newMux())
}

func TestAPI_StripeWebhook_ValidJSON_NoContent(t *testing.T) {
	t.Parallel()

	body := `{"id":"evt_test_123","type":"invoice.payment_succeeded","object":"event"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", "t=1,v1=fake")

	rec := httptest.NewRecorder()
	apiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAPI_StripeWebhook_InvalidJSON_BadRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	apiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAPI_StripeWebhook_WrongMethod(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/webhooks/stripe", nil)
	rec := httptest.NewRecorder()
	apiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /webhooks/stripe: status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestAPI_Livez(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	apiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /livez: status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestAPI_Readyz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	apiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /readyz: status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
}
