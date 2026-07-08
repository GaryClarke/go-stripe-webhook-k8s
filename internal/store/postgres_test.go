package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// testPostgres opens a real Postgres against stripe_webhook_test.
// Skips (does not fail) when Compose is down or migrations were not applied.
func testPostgres(t *testing.T) *Postgres {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL_TEST")
	if dsn == "" {
		// Default matches Makefile DATABASE_URL_TEST when env is unset.
		dsn = "postgres://webhook:webhook@localhost:5433/stripe_webhook_test?sslmode=disable"
	}

	p, err := NewPostgres(dsn)
	if err != nil {
		t.Skipf("integration test skipped: postgres not available (%v); run: make db-up && make db-migrate-test", err)
	}
	return p
}

// resetLedger clears ledger and outbox so each test starts from empty tables.
func resetLedger(t *testing.T, p *Postgres) {
	t.Helper()
	if err := p.TruncateLedger(context.Background()); err != nil {
		t.Fatalf("truncate ledger: %v", err)
	}
}

func TestProcessEvent_claimThenDuplicate(t *testing.T) {
	p := testPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	const (
		eventID   = "evt_integration_claim_dup"
		eventType = "invoice.payment_succeeded"
	)

	// First delivery: INSERT should win the claim and commit status=processed.
	claimed, err := p.ProcessEvent(ctx, eventID, eventType, nil)
	if err != nil {
		t.Fatalf("first ProcessEvent: %v", err)
	}
	if !claimed {
		t.Fatal("first ProcessEvent: want claimed=true")
	}

	// Ledger should show one processed row.
	es, err := p.Status(ctx, eventID)
	if err != nil {
		t.Fatalf("Status after first claim: %v", err)
	}
	if !es.Found {
		t.Fatal("Status: want Found=true after first claim")
	}
	if es.Status != StatusProcessed {
		t.Fatalf("Status = %q, want %q", es.Status, StatusProcessed)
	}

	// Second delivery (Stripe retry / duplicate): ON CONFLICT → no new insert.
	claimed, err = p.ProcessEvent(ctx, eventID, eventType, nil)
	if err != nil {
		t.Fatalf("second ProcessEvent: %v", err)
	}
	if claimed {
		t.Fatal("second ProcessEvent: want claimed=false on duplicate event_id")
	}

	// Still exactly one row, still processed (not a second insert).
	var count int
	if err := p.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM processed_events WHERE event_id = $1`,
		eventID,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func TestAcceptEvent_claimThenDuplicate(t *testing.T) {
	p := testPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	const (
		eventID   = "evt_accept_claim_dup"
		eventType = "invoice.payment_succeeded"
	)
	jobPayload := []byte(`{"stripe_event_id":"evt_accept_claim_dup","event_type":"invoice.payment_succeeded","payload":{}}`)

	claimed, err := p.AcceptEvent(ctx, eventID, eventType, jobPayload)
	if err != nil {
		t.Fatalf("first AcceptEvent: %v", err)
	}
	if !claimed {
		t.Fatal("first AcceptEvent: want claimed=true")
	}

	es, err := p.Status(ctx, eventID)
	if err != nil {
		t.Fatalf("Status after first accept: %v", err)
	}
	if !es.Found {
		t.Fatal("Status: want Found=true after first accept")
	}
	if es.Status != StatusAccepted {
		t.Fatalf("Status = %q, want %q", es.Status, StatusAccepted)
	}

	ob, err := p.OutboxStatus(ctx, eventID)
	if err != nil {
		t.Fatalf("OutboxStatus after first accept: %v", err)
	}
	if !ob.Found {
		t.Fatal("OutboxStatus: want Found=true after first accept")
	}
	if ob.Status != OutboxPending {
		t.Fatalf("OutboxStatus status = %q, want %q", ob.Status, OutboxPending)
	}
	var storedJob struct {
		StripeEventID string `json:"stripe_event_id"`
		EventType     string `json:"event_type"`
	}
	if err := json.Unmarshal(ob.Payload, &storedJob); err != nil {
		t.Fatalf("unmarshal outbox payload: %v", err)
	}
	if storedJob.StripeEventID != eventID {
		t.Fatalf("outbox stripe_event_id = %q, want %q", storedJob.StripeEventID, eventID)
	}
	if storedJob.EventType != eventType {
		t.Fatalf("outbox event_type = %q, want %q", storedJob.EventType, eventType)
	}

	claimed, err = p.AcceptEvent(ctx, eventID, eventType, jobPayload)
	if err != nil {
		t.Fatalf("second AcceptEvent: %v", err)
	}
	if claimed {
		t.Fatal("second AcceptEvent: want claimed=false on duplicate event_id")
	}

	es, err = p.Status(ctx, eventID)
	if err != nil {
		t.Fatalf("Status after duplicate: %v", err)
	}
	if es.Status != StatusAccepted {
		t.Fatalf("Status after duplicate = %q, want %q", es.Status, StatusAccepted)
	}

	ob, err = p.OutboxStatus(ctx, eventID)
	if err != nil {
		t.Fatalf("OutboxStatus after duplicate: %v", err)
	}
	if ob.Status != OutboxPending {
		t.Fatalf("OutboxStatus after duplicate = %q, want %q", ob.Status, OutboxPending)
	}

	var ledgerCount, outboxCount int
	if err := p.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM processed_events WHERE event_id = $1`, eventID,
	).Scan(&ledgerCount); err != nil {
		t.Fatalf("count ledger rows: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("ledger row count = %d, want 1", ledgerCount)
	}
	if err := p.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_id = $1`, eventID,
	).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox row count = %d, want 1", outboxCount)
	}
}

func TestStatus_notFound(t *testing.T) {
	p := testPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	// No row for this event_id after truncate.
	es, err := p.Status(ctx, "evt_does_not_exist")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if es.Found {
		t.Fatal("Status: want Found=false when event_id absent")
	}
}
