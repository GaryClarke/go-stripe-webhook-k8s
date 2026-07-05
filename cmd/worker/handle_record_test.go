package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestHandleRecord(t *testing.T) {
	cases := []struct {
		name       string
		value      string
		partition  int32
		offset     int64
		wantOK     bool
		wantMsg    string
		wantFields map[string]any
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
			wantOK:  true,
			wantMsg: stripeJobConsumed,
			wantFields: map[string]any{
				"event_id":   "",
				"event_type": "",
				"partition":  float64(0),
				"offset":     float64(0),
			},
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

			gotOK := handleRecord(logger, rec)
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
