package store

import "context"

// Status values for processed_events.status (must match DB CHECK constraint).
const (
	StatusProcessing = "processing"
	StatusAccepted   = "accepted" // M9: durably accepted for async processing
	StatusProcessed  = "processed"
	StatusFailed     = "failed"
)

// Outbox status values for outbox_events.status (must match DB CHECK constraint).
const (
	OutboxPending   = "pending"
	OutboxPublished = "published"
	OutboxFailed    = "failed"
)

// EventStatus is the ledger summary for one Stripe event.id.
type EventStatus struct {
	Status string // processing | processed | failed
	Found  bool   // false when no row exists for event_id
}

// OutboxStatus is the outbox row summary for one event_id (Phase 4 tests / publisher later).
type OutboxStatus struct {
	Found   bool
	Status  string
	Payload []byte
}

// Store persists webhook idempotency state in Postgres.
// Implementations must be safe for concurrent use across multiple Pods.
type Store interface {
	// ProcessEvent claims eventID in a transaction when new.
	// If this caller wins the claim, fn runs inside the same transaction, then status becomes processed.
	// Returns claimed=false when eventID already exists (use Status for logging).
	ProcessEvent(ctx context.Context, eventID, eventType string, fn func(ctx context.Context) error) (claimed bool, err error)

	// AcceptEvent claims eventID, inserts outbox_events (pending), marks ledger accepted — one transaction.
	// jobPayload is JSON for engine.Job (caller marshals; store stays Stripe-free).
	// Returns claimed=false when eventID already exists (not an error).
	AcceptEvent(ctx context.Context, eventID, eventType string, jobPayload []byte) (claimed bool, err error)

	// Status returns ledger state for eventID. Found=false when no row exists.
	Status(ctx context.Context, eventID string) (*EventStatus, error)

	// OutboxStatus returns outbox row for eventID. Found=false when no outbox row exists.
	OutboxStatus(ctx context.Context, eventID string) (*OutboxStatus, error)

	// Ping checks database connectivity (for /readyz).
	Ping(ctx context.Context) error
}
