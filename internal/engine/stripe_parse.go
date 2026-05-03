package engine

import (
	"encoding/json"
	"fmt"
)

// ParseStripeEvent decodes a Stripe webhook request body into a StripeEvent.
// The caller supplies raw JSON bytes (e.g. from API Gateway event.Body, or ReadAll(r.Body)).
func ParseStripeEvent(body []byte) (StripeEvent, error) {
	var ev StripeEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return StripeEvent{}, fmt.Errorf("parse stripe event: %w", err)
	}
	return ev, nil
}
