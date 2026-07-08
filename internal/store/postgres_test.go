package store

import (
	"context"
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
