package engine

import "encoding/json"

// Job is the internal representation of work to forward downstream.
// It is enqueued by the ingest handler and consumed by the worker.
type Job struct {
	// StripeEventID is the Stripe event ID (e.g. evt_xxx). Used for idempotency
	// (avoid processing the same event twice) and for tracing.
	StripeEventID string `json:"stripe_event_id"`

	// EventType is the Stripe event type (e.g. invoice.payment_succeeded).
	// Lets the worker decide how to handle or route the payload.
	EventType string `json:"event_type"`

	// Payload is the data to forward (typically data.object from the Stripe event).
	// Kept as raw JSON so we can pass through unchanged or unmarshal when needed.
	Payload json.RawMessage `json:"payload"`
}

// JobFromStripeEvent builds a Job for the queue from a parsed webhook event.
func JobFromStripeEvent(evt StripeEvent) Job {
	return Job{
		StripeEventID: evt.ID,
		EventType:     evt.Type,
		Payload:       evt.Data.Object,
	}
}
