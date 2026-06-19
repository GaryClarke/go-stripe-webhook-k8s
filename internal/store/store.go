package store

import "context"

// Status values for processed_events.status (must match DB CHECK constraint).
const (
	StatusProcessing = "processing"
	StatusProcessed  = "processed"
	StatusFailed     = "failed"
)

// EventStatus is the ledger summary for one Stripe event.id.
type EventStatus struct {
	Status string // processing | processed | failed
	Found  bool   // false when no row exists for event_id
}

// Store persists webhook idempotency state in Postgres.
// Implementations must be safe for concurrent use across multiple Pods.
type Store interface {
	// ProcessEvent claims eventID in a transaction when new.
	// If this caller wins the claim, fn runs inside the same transaction, then status becomes processed.
	// Returns claimed=false when eventID already exists (use Status for logging).
	ProcessEvent(ctx context.Context, eventID, eventType string, fn func(ctx context.Context) error) (claimed bool, err error)

	// Status returns ledger state for eventID. Found=false when no row exists.
	Status(ctx context.Context, eventID string) (*EventStatus, error)

	// Ping checks database connectivity (for /readyz).
	Ping(ctx context.Context) error
}
