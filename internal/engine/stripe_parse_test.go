package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseStripeEvent_fixture(t *testing.T) {
	t.Parallel()

	// testdata lives at repo root; tests run with cwd = this package directory.
	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "stripe-invoice-payment-succeeded.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	ev, err := ParseStripeEvent(body)
	if err != nil {
		t.Fatalf("ParseStripeEvent: %v", err)
	}

	if ev.ID != "evt_1TCjyAIq4hctS9aMRNWEXxN2" {
		t.Errorf("ID = %q, want evt_1TCjyAIq4hctS9aMRNWEXxN2", ev.ID)
	}
	if ev.Type != "invoice.payment_succeeded" {
		t.Errorf("Type = %q, want invoice.payment_succeeded", ev.Type)
	}
	if ev.Created != 1773939769 {
		t.Errorf("Created = %d, want 1773939769", ev.Created)
	}
	if ev.Livemode != false {
		t.Errorf("Livemode = %v, want false", ev.Livemode)
	}
	if len(ev.Data.Object) == 0 {
		t.Error("Data.Object is empty")
	}
}

func TestParseStripeEvent_invalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseStripeEvent([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
