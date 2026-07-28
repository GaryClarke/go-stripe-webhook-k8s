package main

import (
	"bytes"
	"context"
	"encoding/json"
	"integration-engine/internal/engine"
	"integration-engine/internal/store"
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

type fakeCompletionStore struct {
	claimAction store.CompletionClaimAction
}

func (f *fakeCompletionStore) ClaimConsumerCompletion(
	ctx context.Context, eventID, consumerName, eventType string,
) (*store.CompletionClaim, error) {
	action := f.claimAction
	if action == "" {
		action = store.CompletionClaimNew
	}
	return &store.CompletionClaim{Action: action, AttemptCount: 1}, nil
}
func (f *fakeCompletionStore) MarkConsumerProcessed(ctx context.Context, eventID, consumerName string) (bool, error) {
	return true, nil
}
func (f *fakeCompletionStore) MarkConsumerFailed(ctx context.Context, eventID, consumerName, errMsg string) (bool, error) {
	return true, nil
}

type fakeDownstream struct {
	err   error
	calls int
}

func (f *fakeDownstream) DeliverJob(_ context.Context, _ engine.Job) error {
	f.calls++
	return f.err
}

func TestHandleRecord(t *testing.T) {
	cases := []struct {
		name          string
		value         string
		claimAction   store.CompletionClaimAction
		partition     int32
		offset        int64
		wantOK        bool
		wantMsg       string
		wantFields          map[string]any
		downstreamErr       error
		wantDownstreamCalls *int
	}{
		{
			name:    "valid full job",
			value:   `{"stripe_event_id":"evt_1","event_type":"invoice.payment_succeeded","payload":{"id":"in_123"}}`,
			wantOK:  true,
			wantMsg: stripeJobConsumed,
			wantFields: map[string]any{
				"event_id":   "evt_1",
				"event_type": "invoice.payment_succeeded",
				"partition":  float64(0),
				"offset":     float64(10),
			},
			partition: 0,
			offset:    10,
		},
		{
			name:    "minimal job empty event_type",
			value:   `{"stripe_event_id":"evt_smoke"}`,
			wantOK:  true,
			wantMsg: stripeJobConsumed,
			wantFields: map[string]any{
				"event_id":   "evt_smoke",
				"event_type": "",
				"partition":  float64(1),
				"offset":     float64(2),
			},
			partition: 1,
			offset:    2,
		},
		{
			name:    "bad json",
			value:   `not json`,
			wantOK:  false,
			wantMsg: stripeJobUnmarshalFailed,
		},
		{
			name:    "empty object",
			value:   `{}`,
			wantOK:  false,
			wantMsg: stripeJobUnmarshalFailed,
			wantFields: map[string]any{
				"error": "missing stripe_event_id",
			},
		},
		{
			name:        "duplicate skip",
			value:       `{"stripe_event_id":"evt_dup","event_type":"invoice.payment_succeeded"}`,
			claimAction: store.CompletionClaimAlreadyProcessed,
			wantOK:      true,
			wantMsg:     stripeJobDuplicateSkipped,
			wantDownstreamCalls: new(int),
		},
		{
			name:          "downstream retryable 503",
			value:         `{"stripe_event_id":"evt_503","event_type":"invoice.payment_succeeded"}`,
			downstreamErr: classifyHTTPStatus(503),
			wantOK:        false,
			wantMsg:       stripeJobDownstreamFailed,
		},
		{
			name:          "downstream permanent 400",
			value:         `{"stripe_event_id":"evt_400","event_type":"invoice.payment_succeeded"}`,
			downstreamErr: classifyHTTPStatus(400),
			wantOK:        true, // commit offset after failed row
			wantMsg:       stripeJobDownstreamFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewJSONLogger(&buf, nil)

			rec := &kgo.Record{
				Partition: tc.partition,
				Offset:    tc.offset,
				Value:     []byte(tc.value),
			}

			st := &fakeCompletionStore{claimAction: tc.claimAction}
			ds := &fakeDownstream{err: tc.downstreamErr}

			gotOK := handleRecord(context.Background(), logger, st, ds, "stripe-webhook-worker", rec)
			if gotOK != tc.wantOK {
				t.Fatalf("handleRecord() = %v, want %v", gotOK, tc.wantOK)
			}

			fields := lastLogFields(t, &buf)
			if fields["msg"] != tc.wantMsg {
				t.Fatalf("msg = %v, want %q", fields["msg"], tc.wantMsg)
			}

			for key, want := range tc.wantFields {
				if fields[key] != want {
					t.Fatalf("%s = %v, want %v", key, fields[key], want)
				}
			}

			if tc.wantDownstreamCalls != nil && ds.calls != *tc.wantDownstreamCalls {
				t.Fatalf("DeliverJob calls = %d, want %d", ds.calls, *tc.wantDownstreamCalls)
			}
		})
	}
}

func lastLogFields(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		t.Fatal("expected at least one log line, got empty buffer")
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &fields); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, lines[len(lines)-1])
	}
	return fields
}
