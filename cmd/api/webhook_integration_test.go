package main

import (
	"context"
	"integration-engine/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func testPostgresStore(t *testing.T) *store.Postgres {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL_TEST")
	if dsn == "" {
		dsn = "postgres://webhook:webhook@localhost:5433/stripe_webhook_test?sslmode=disable"
	}

	p, err := store.NewPostgres(dsn)
	if err != nil {
		t.Skipf("integration test skipped: postgres not available (%v); run: make db-up && make db-migrate-test", err)
	}
	return p
}

func TestAPI_StripeWebhook_DuplicateDelivery_Integration(t *testing.T) {
	// Do not t.Parallel — shares one Postgres database.
	ctx := context.Background()
	p := testPostgresStore(t)

	if err := p.TruncateLedger(ctx); err != nil {
		t.Fatalf("truncate ledger: %v", err)
	}

	const eventID = "evt_integration_webhook_dup"
	body := webhookJSON(eventID)
	handler, logBuf := newAPIHandler(p)

	req1 := newSignedStripeWebhookRequest(body)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusNoContent {
		t.Fatalf("first POST: status = %d, want %d", rec1.Code, http.StatusNoContent)
	}

	ob, err := p.OutboxStatus(ctx, eventID)
	if err != nil {
		t.Fatalf("OutboxStatus after first accept: %v", err)
	}
	if !ob.Found || ob.Status != store.OutboxPending {
		t.Fatalf("outbox after first accept = %+v, want Found=true Status=pending", ob)
	}

	req2 := newSignedStripeWebhookRequest(body)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("second POST: status = %d, want %d", rec2.Code, http.StatusNoContent)
	}

	msgs := logMsgs(logBuf)
	if got := countMsg(msgs, msgStripeEventAccepted); got != 1 {
		t.Fatalf("stripe_event_accepted count = %d, want 1; msgs = %v", got, msgs)
	}
	if got := countMsg(msgs, msgStripeEventDuplicateSkipped); got != 1 {
		t.Fatalf("stripe_event_duplicate_skipped count = %d, want 1; msgs = %v", got, msgs)
	}

	es, err := p.Status(ctx, eventID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !es.Found || es.Status != store.StatusAccepted {
		t.Fatalf("ledger status = %+v, want Found=true Status=accepted", es)
	}
}

func TestAPI_StripeWebhook_ConcurrentDuplicate_Integration(t *testing.T) {
	// Do not t.Parallel — shares one Postgres database.
	ctx := context.Background()
	p := testPostgresStore(t)

	if err := p.TruncateLedger(ctx); err != nil {
		t.Fatalf("truncate ledger: %v", err)
	}

	const eventID = "evt_integration_webhook_race"
	body := webhookJSON(eventID)
	handler, logBuf := newAPIHandler(p)

	const n = 8
	start := make(chan struct{})
	done := make(chan int, n)

	for range n {
		go func() {
			<-start // wait until all goroutines are ready
			req := newSignedStripeWebhookRequest(body)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			done <- rec.Code
		}()
	}
	close(start) // release all goroutines together

	for range n {
		if code := <-done; code != http.StatusNoContent {
			t.Fatalf("concurrent POST: status = %d, want %d", code, http.StatusNoContent)
		}
	}

	msgs := logMsgs(logBuf)
	if got := countMsg(msgs, msgStripeEventAccepted); got != 1 {
		t.Fatalf("stripe_event_accepted count = %d, want 1; msgs = %v", got, msgs)
	}
	dup := countMsg(msgs, msgStripeEventDuplicateSkipped)
	inFlight := countMsg(msgs, msgStripeEventAlreadyProcessing)
	if dup+inFlight != n-1 {
		t.Fatalf("duplicate/in-flight count = %d+%d, want %d; msgs = %v", dup, inFlight, n-1, msgs)
	}

	es, err := p.Status(ctx, eventID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !es.Found || es.Status != store.StatusAccepted {
		t.Fatalf("ledger status = %+v, want Found=true Status=accepted", es)
	}

	ob, err := p.OutboxStatus(ctx, eventID)
	if err != nil {
		t.Fatalf("OutboxStatus after race: %v", err)
	}
	if !ob.Found || ob.Status != store.OutboxPending {
		t.Fatalf("outbox after race = %+v, want Found=true Status=pending", ob)
	}
}
