//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"integration-engine/internal/store"
	"os"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func openTestPostgres(t *testing.T) *store.Postgres {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL_TEST")
	if dsn == "" {
		// Default matches Makefile DATABASE_URL_TEST when env is unset.
		dsn = "postgres://webhook:webhook@localhost:5433/stripe_webhook_test?sslmode=disable"
	}

	p, err := store.NewPostgres(dsn)
	if err != nil {
		t.Skipf("integration test skipped: postgres not available (%v); run: make db-up && make db-migrate-test", err)
	}
	return p
}

func TestHandleRecord_WritesCompletionRow_Integration(t *testing.T) {
	pg := openTestPostgres(t)
	ctx := context.Background()

	if err := pg.TruncateLedger(ctx); err != nil {
		t.Fatalf("TruncateLedger: %v", err)
	}

	const (
		eventID      = "evt_worker_completion_integration"
		eventType    = "invoice.payment_succeeded"
		consumerName = "stripe-webhook-worker"
	)

	value, err := json.Marshal(map[string]any{
		"stripe_event_id": eventID,
		"event_type":      eventType,
		"payload":         map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}

	rec := &kgo.Record{
		Partition: 0,
		Offset:    1,
		Value:     value,
	}

	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)

	ok := handleRecord(ctx, logger, pg, consumerName, rec)
	if !ok {
		t.Fatal("handleRecord: want true")
	}

	cs, err := pg.CompletionStatus(ctx, eventID, consumerName)
	if err != nil {
		t.Fatalf("CompletionStatus: %v", err)
	}
	if !cs.Found {
		t.Fatalf("CompletionStatus: row not found for %q", eventID)
	}
	if cs.Status != store.CompletionProcessed {
		t.Fatalf("CompletionStatus: status = %q, want %q", cs.Status, store.CompletionProcessed)
	}
	if cs.AttemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", cs.AttemptCount)
	}
}

func TestHandleRecord_DuplicateSkip_Integration(t *testing.T) {
	pg := openTestPostgres(t)
	ctx := context.Background()

	if err := pg.TruncateLedger(ctx); err != nil {
		t.Fatalf("TruncateLedger: %v", err)
	}

	const (
		eventID      = "evt_worker_duplicate_integration"
		eventType    = "invoice.payment_succeeded"
		consumerName = "stripe-webhook-worker"
	)

	value, err := json.Marshal(map[string]any{
		"stripe_event_id": eventID,
		"event_type":      eventType,
	})
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}

	rec := &kgo.Record{
		Partition: 0,
		Offset:    2,
		Value:     value,
	}

	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)

	if ok := handleRecord(ctx, logger, pg, consumerName, rec); !ok {
		t.Fatal("first handleRecord: want true")
	}

	buf.Reset()
	if ok := handleRecord(ctx, logger, pg, consumerName, rec); !ok {
		t.Fatal("second handleRecord: want true (duplicate skip is safe to commit)")
	}

	fields := lastLogFields(t, &buf)
	if fields["msg"] != stripeJobDuplicateSkipped {
		t.Fatalf("second handle msg = %v, want %q", fields["msg"], stripeJobDuplicateSkipped)
	}

	cs, err := pg.CompletionStatus(ctx, eventID, consumerName)
	if err != nil {
		t.Fatalf("CompletionStatus: %v", err)
	}
	if cs.Status != store.CompletionProcessed {
		t.Fatalf("status = %q, want %q after duplicate", cs.Status, store.CompletionProcessed)
	}
}
