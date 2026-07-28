package main

import (
	"context"
	"encoding/json"
	"errors"
	"integration-engine/internal/engine"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPDownstream_DeliverJob(t *testing.T) {
	job := engine.Job{
		StripeEventID: "evt_test",
		EventType:     "invoice.payment_succeeded",
		Payload:       json.RawMessage(`{"id":"in_123"}`),
	}

	cases := []struct {
		name       string
		statusCode int
		wantErr    bool
		retryable  bool
		httpStatus int
	}{
		{
			name:       "200 success",
			statusCode: http.StatusOK,
		},
		{
			name:       "503 retryable",
			statusCode: http.StatusServiceUnavailable,
			wantErr:    true,
			retryable:  true,
			httpStatus: 503,
		},
		{
			name:       "400 permanent",
			statusCode: http.StatusBadRequest,
			wantErr:    true,
			retryable:  false,
			httpStatus: 400,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", ct)
				}

				// On success case, assert request body is the job JSON.
				if tc.statusCode == http.StatusOK {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("read body: %v", err)
					}
					var got engine.Job
					if err := json.Unmarshal(body, &got); err != nil {
						t.Fatalf("unmarshal body: %v", err)
					}
					if got.StripeEventID != job.StripeEventID {
						t.Errorf("stripe_event_id = %q, want %q", got.StripeEventID, job.StripeEventID)
					}
				}

				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			client := NewHTTPDownstream(srv.URL, srv.Client())
			err := client.DeliverJob(context.Background(), job)

			if tc.wantErr {
				if err == nil {
					t.Fatal("DeliverJob: err = nil, want error")
				}
				je, ok := errors.AsType[*JobError](err)
				if !ok {
					t.Fatalf("error type = %T, want *JobError", err)
				}
				if je.Retryable != tc.retryable {
					t.Fatalf("Retryable = %v, want %v", je.Retryable, tc.retryable)
				}
				if je.HTTPStatus != tc.httpStatus {
					t.Fatalf("HTTPStatus = %d, want %d", je.HTTPStatus, tc.httpStatus)
				}
				return
			}

			if err != nil {
				t.Fatalf("DeliverJob: %v", err)
			}
		})
	}
}
