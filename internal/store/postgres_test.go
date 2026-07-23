package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

const testConsumerName = "stripe-webhook-worker"

// openTestPostgres opens a real Postgres against stripe_webhook_test.
// Skips (does not fail) when Compose is down or migrations were not applied.
func openTestPostgres(t *testing.T) *Postgres {
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
	p := openTestPostgres(t)
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
	p := openTestPostgres(t)
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

func TestOutboxPublisher_nextThenMarkPublished(t *testing.T) {
	p := openTestPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	const (
		eventID   = "evt_outbox_publish"
		eventType = "invoice.payment_succeeded"
	)
	jobPayload := []byte(`{"stripe_event_id":"evt_outbox_publish","event_type":"invoice.payment_succeeded","payload":{}}`)

	claimed, err := p.AcceptEvent(ctx, eventID, eventType, jobPayload)
	if err != nil || !claimed {
		t.Fatalf("AcceptEvent: claimed=%v err=%v", claimed, err)
	}

	row, err := p.NextPendingOutbox(ctx)
	if err != nil {
		t.Fatalf("NextPendingOutbox: %v", err)
	}
	if row == nil {
		t.Fatal("NextPendingOutbox: want row, got nil")
	}
	if row.EventID != eventID {
		t.Fatalf("EventID = %q, want %q", row.EventID, eventID)
	}

	updated, err := p.MarkOutboxPublished(ctx, eventID)
	if err != nil {
		t.Fatalf("MarkOutboxPublished: %v", err)
	}
	if !updated {
		t.Fatal("MarkOutboxPublished: want updated=true")
	}

	ob, err := p.OutboxStatus(ctx, eventID)
	if err != nil {
		t.Fatalf("OutboxStatus: %v", err)
	}
	if !ob.Found || ob.Status != OutboxPublished {
		t.Fatalf("outbox = %+v, want published", ob)
	}

	row, err = p.NextPendingOutbox(ctx)
	if err != nil {
		t.Fatalf("NextPendingOutbox after mark: %v", err)
	}
	if row != nil {
		t.Fatalf("NextPendingOutbox after mark = %+v, want nil", row)
	}

	// Idempotent mark: already published.
	updated, err = p.MarkOutboxPublished(ctx, eventID)
	if err != nil {
		t.Fatalf("second MarkOutboxPublished: %v", err)
	}
	if updated {
		t.Fatal("second MarkOutboxPublished: want updated=false")
	}
}

func TestStatus_notFound(t *testing.T) {
	p := openTestPostgres(t)
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

func TestConsumerCompletion_claimThenMarkProcessed(t *testing.T) {
	p := openTestPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	const (
		eventID   = "evt_completion_happy"
		eventType = "invoice.payment_succeeded"
	)
	claim, err := p.ClaimConsumerCompletion(ctx, eventID, testConsumerName, eventType)
	if err != nil {
		t.Fatalf("ClaimConsumerCompletion: %v", err)
	}
	if claim.Action != CompletionClaimNew {
		t.Fatalf("claim action = %q, want %q", claim.Action, CompletionClaimNew)
	}
	if claim.AttemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", claim.AttemptCount)
	}

	cs, err := p.CompletionStatus(ctx, eventID, testConsumerName)
	if err != nil {
		t.Fatalf("CompletionStatus after claim: %v", err)
	}
	if !cs.Found || cs.Status != CompletionProcessing {
		t.Fatalf("CompletionStatus after claim = %+v, want processing", cs)
	}

	updated, err := p.MarkConsumerProcessed(ctx, eventID, testConsumerName)
	if err != nil {
		t.Fatalf("MarkConsumerProcessed: %v", err)
	}
	if !updated {
		t.Fatal("MarkConsumerProcessed: want updated=true")
	}

	cs, err = p.CompletionStatus(ctx, eventID, testConsumerName)
	if err != nil {
		t.Fatalf("CompletionStatus after mark: %v", err)
	}
	if !cs.Found || cs.Status != CompletionProcessed {
		t.Fatalf("CompletionStatus after mark = %+v, want processed", cs)
	}
}

func TestConsumerCompletion_claimAlreadyProcessed(t *testing.T) {
	p := openTestPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	const (
		eventID   = "evt_completion_dup"
		eventType = "invoice.payment_succeeded"
	)

	claim, err := p.ClaimConsumerCompletion(ctx, eventID, testConsumerName, eventType)
	if err != nil || claim.Action != CompletionClaimNew {
		t.Fatalf("first claim = %+v err=%v", claim, err)
	}

	updated, err := p.MarkConsumerProcessed(ctx, eventID, testConsumerName)
	if err != nil || !updated {
		t.Fatalf("MarkConsumerProcessed: updated=%v err=%v", updated, err)
	}

	claim, err = p.ClaimConsumerCompletion(ctx, eventID, testConsumerName, eventType)
	if err != nil {
		t.Fatalf("second ClaimConsumerCompletion: %v", err)
	}
	if claim.Action != CompletionClaimAlreadyProcessed {
		t.Fatalf("second claim action = %q, want %q", claim.Action, CompletionClaimAlreadyProcessed)
	}
}

func TestConsumerCompletion_retryFromProcessing(t *testing.T) {
	p := openTestPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	const (
		eventID   = "evt_completion_retry"
		eventType = "invoice.payment_succeeded"
	)

	claim, err := p.ClaimConsumerCompletion(ctx, eventID, testConsumerName, eventType)
	if err != nil || claim.Action != CompletionClaimNew {
		t.Fatalf("first claim = %+v err=%v", claim, err)
	}

	claim, err = p.ClaimConsumerCompletion(ctx, eventID, testConsumerName, eventType)
	if err != nil {
		t.Fatalf("second ClaimConsumerCompletion: %v", err)
	}
	if claim.Action != CompletionClaimRetry {
		t.Fatalf("second claim action = %q, want %q", claim.Action, CompletionClaimRetry)
	}
	if claim.AttemptCount != 2 {
		t.Fatalf("attempt_count = %d, want 2", claim.AttemptCount)
	}

	cs, err := p.CompletionStatus(ctx, eventID, testConsumerName)
	if err != nil {
		t.Fatalf("CompletionStatus: %v", err)
	}
	if cs.Status != CompletionProcessing {
		t.Fatalf("status = %q, want %q", cs.Status, CompletionProcessing)
	}
}

func TestConsumerCompletion_retryFromFailed(t *testing.T) {
	p := openTestPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	const (
		eventID   = "evt_completion_retry_failed"
		eventType = "invoice.payment_succeeded"
		failMsg   = "downstream timeout"
	)

	claim, err := p.ClaimConsumerCompletion(ctx, eventID, testConsumerName, eventType)
	if err != nil || claim.Action != CompletionClaimNew {
		t.Fatalf("first claim = %+v err=%v", claim, err)
	}

	updated, err := p.MarkConsumerFailed(ctx, eventID, testConsumerName, failMsg)
	if err != nil || !updated {
		t.Fatalf("MarkConsumerFailed: updated=%v err=%v", updated, err)
	}

	cs, err := p.CompletionStatus(ctx, eventID, testConsumerName)
	if err != nil {
		t.Fatalf("CompletionStatus after fail: %v", err)
	}
	if cs.Status != CompletionFailed || cs.Error != failMsg {
		t.Fatalf("CompletionStatus after fail = %+v, want failed with error", cs)
	}

	claim, err = p.ClaimConsumerCompletion(ctx, eventID, testConsumerName, eventType)
	if err != nil {
		t.Fatalf("reclaim after failed: %v", err)
	}
	if claim.Action != CompletionClaimRetryFromFailed {
		t.Fatalf("reclaim action = %q, want %q", claim.Action, CompletionClaimRetryFromFailed)
	}
	if claim.AttemptCount != 2 {
		t.Fatalf("attempt_count = %d, want 2", claim.AttemptCount)
	}

	cs, err = p.CompletionStatus(ctx, eventID, testConsumerName)
	if err != nil {
		t.Fatalf("CompletionStatus after reclaim: %v", err)
	}
	if cs.Status != CompletionProcessing {
		t.Fatalf("status = %q, want %q", cs.Status, CompletionProcessing)
	}
	if cs.Error != "" {
		t.Fatalf("error = %q, want cleared", cs.Error)
	}
	if cs.AttemptCount != 2 {
		t.Fatalf("attempt_count = %d, want 2", cs.AttemptCount)
	}
}

func TestConsumerCompletion_markProcessedIdempotent(t *testing.T) {
	p := openTestPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	const (
		eventID   = "evt_completion_idempotent"
		eventType = "invoice.payment_succeeded"
	)

	claim, err := p.ClaimConsumerCompletion(ctx, eventID, testConsumerName, eventType)
	if err != nil || claim.Action != CompletionClaimNew {
		t.Fatalf("ClaimConsumerCompletion: %+v err=%v", claim, err)
	}

	updated, err := p.MarkConsumerProcessed(ctx, eventID, testConsumerName)
	if err != nil || !updated {
		t.Fatalf("first MarkConsumerProcessed: updated=%v err=%v", updated, err)
	}

	updated, err = p.MarkConsumerProcessed(ctx, eventID, testConsumerName)
	if err != nil {
		t.Fatalf("second MarkConsumerProcessed: %v", err)
	}
	if updated {
		t.Fatal("second MarkConsumerProcessed: want updated=false")
	}

	cs, err := p.CompletionStatus(ctx, eventID, testConsumerName)
	if err != nil {
		t.Fatalf("CompletionStatus: %v", err)
	}
	if !cs.Found || cs.Status != CompletionProcessed {
		t.Fatalf("CompletionStatus = %+v, want processed", cs)
	}
}

func TestCompletionStatus_notFound(t *testing.T) {
	p := openTestPostgres(t)
	resetLedger(t, p)
	ctx := context.Background()

	cs, err := p.CompletionStatus(ctx, "evt_completion_missing", testConsumerName)
	if err != nil {
		t.Fatalf("CompletionStatus: %v", err)
	}
	if cs.Found {
		t.Fatalf("CompletionStatus = %+v, want Found=false", cs)
	}
}
