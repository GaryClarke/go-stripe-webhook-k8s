package engine

import "encoding/json"

// StripeEvent is the top-level webhook payload from Stripe.
// See https://docs.stripe.com/api/events/object
type StripeEvent struct {
	// ID is the unique event identifier (e.g. evt_xxx). Use for idempotency and tracing.
	ID string `json:"id"`

	// Type is the event name (e.g. invoice.payment_succeeded, invoice.payment_failed).
	// Drives routing and handling logic.
	Type string `json:"type"`

	// Data contains the event payload. Object holds the resource (invoice, charge, etc.).
	Data StripeEventData `json:"data"`

	// Created is the Unix timestamp when Stripe created the event.
	Created int64 `json:"created"`

	// Livemode is true for live-mode events, false for test mode.
	Livemode bool `json:"livemode"`
}

// StripeEventData wraps the data object from a Stripe event.
type StripeEventData struct {
	// Object is the API resource (invoice, charge, etc.). Kept as raw JSON so we can
	// unmarshal into specific types later or forward as-is to downstream.
	Object json.RawMessage `json:"object"`
}
