package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type pingFailStore struct {
	fakeStore
}

func (f *pingFailStore) Ping(ctx context.Context) error {
	return errors.New("db unreachable")
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

// newAPIHandler wires the same stack as main but lets tests inject the store
// and capture JSON logs from the buffer.
func newAPIHandler(st store.Store) (http.Handler, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)
	app := NewApp(&config.Config{
		StripeWebhookSecret: testWebhookSecret,
		Port:                "8080",
	}, logger, st)
	return Recover(logger, RequestLog(logger, app.routes())), &buf
}

// apiHandler matches main for unit tests (fake store, logs discarded).
func apiHandler() http.Handler {
	h, _ := newAPIHandler(newFakeStore())
	return h
}

func webhookJSON(eventID string) string {
	return fmt.Sprintf(
		`{"id":%q,"type":"invoice.payment_succeeded","object":"event","api_version":%q}`,
		eventID,
		stripe.APIVersion,
	)
}

// validStripeWebhookJSON is JSON ConstructEvent accepts for testWebhookSecret when paired with a valid Stripe-Signature.
func validStripeWebhookJSON() string {
	return webhookJSON("evt_test_123")
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

func logMsgs(buf *bytes.Buffer) []string {
	var msgs []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			continue
		}
		if msg, ok := fields["msg"].(string); ok {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

func countMsg(msgs []string, want string) int {
	n := 0
	for _, m := range msgs {
		if m == want {
			n++
		}
	}
	return n
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

func TestAPI_Readyz_DBDown_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h, _ := newAPIHandler(&pingFailStore{})
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz: status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
