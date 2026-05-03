package engine

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJobFromStripeEvent(t *testing.T) {
	t.Parallel()
	evt := StripeEvent{
		ID:   "evt_1TCjyAIq4hctS9aMRNWEXxN2",
		Type: "invoice.payment_succeeded",
		Data: StripeEventData{
			Object: json.RawMessage(`{"id":"in_test"}`),
		},
		Created:  0,
		Livemode: false,
	}

	job := JobFromStripeEvent(evt)

	if job.StripeEventID != evt.ID {
		t.Errorf("job.StripeEventID = %s, want %s", job.StripeEventID, evt.ID)
	}
	if job.EventType != evt.Type {
		t.Errorf("job.EventType = %s, want %s", job.EventType, evt.Type)
	}
	if !bytes.Equal(job.Payload, evt.Data.Object) {
		t.Errorf("job.Payload = %q, want %q", job.Payload, evt.Data.Object)
	}
}
