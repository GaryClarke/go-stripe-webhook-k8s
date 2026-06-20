package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stripe/stripe-go/v85"

	"integration-engine/internal/config"
	"integration-engine/internal/store"
)

const testWebhookSecret = "whsec_test"

type fakeStore struct {
	mu   sync.Mutex
	rows map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: make(map[string]string)}
}

func (f *fakeStore) ProcessEvent(ctx context.Context, eventID, eventType string, fn func(context.Context) error) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[eventID]; ok {
		return false, nil
	}
	f.rows[eventID] = store.StatusProcessing
	if err := fn(ctx); err != nil {
		f.rows[eventID] = store.StatusFailed
		return false, err
	}
	f.rows[eventID] = store.StatusProcessed
	return true, nil
}

func (f *fakeStore) Status(ctx context.Context, eventID string) (*store.EventStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.rows[eventID]
	if !ok {
		return &store.EventStatus{Found: false}, nil
	}
	return &store.EventStatus{Found: true, Status: s}, nil
}

func (f *fakeStore) Ping(ctx context.Context) error {
	return nil
}

// apiHandler matches main: Recover(RequestLog(routes)).
func apiHandler() http.Handler {
	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)
	app := NewApp(&config.Config{
		StripeWebhookSecret: testWebhookSecret,
		Port:                "8080",
	},
		logger, newFakeStore(),
	)
	return Recover(logger, RequestLog(logger, app.routes()))
}

// validStripeWebhookJSON is JSON ConstructEvent accepts for testWebhookSecret when paired with a valid Stripe-Signature.
func validStripeWebhookJSON() string {
	return fmt.Sprintf(
		`{"id":"evt_test_123","type":"invoice.payment_succeeded","object":"event","api_version":%q}`,
		stripe.APIVersion,
	)
}

// newSignedStripeWebhookRequest builds a POST /webhooks/stripe request with a valid
// Stripe-Signature for testWebhookSecret (see stripe.GenerateTestSignedPayload).
func newSignedStripeWebhookRequest(body string) *http.Request {
	p := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{
		Payload: []byte(body),
		Secret:  testWebhookSecret,
	})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", p.Header)
	return req
}

func TestAPI_StripeWebhook_ValidJSON_NoContent(t *testing.T) {
	t.Parallel()

	req := newSignedStripeWebhookRequest(validStripeWebhookJSON())

	rec := httptest.NewRecorder()
	apiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAPI_StripeWebhook_MissingSignature_BadRequest(t *testing.T) {
	t.Parallel()

	body := validStripeWebhookJSON()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	apiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (missing Stripe-Signature must not succeed)", rec.Code, http.StatusBadRequest)
	}
}

func TestAPI_StripeWebhook_InvalidSignature_BadRequest(t *testing.T) {
	t.Parallel()

	body := validStripeWebhookJSON()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", "t=1,v1=fake")

	rec := httptest.NewRecorder()
	apiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (invalid Stripe-Signature must not succeed)", rec.Code, http.StatusBadRequest)
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
