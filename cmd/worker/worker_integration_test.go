//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"integration-engine/internal/engine"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func testKafkaBrokers(t *testing.T) []string {
	t.Helper()

	raw := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
	if raw == "" {
		raw = "localhost:19092"
	}

	brokers := strings.Split(raw, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Skipf("integration test skipped: kafka client init failed (%v); run: make kafka-up", err)
	}
	defer cl.Close()

	if err := cl.Ping(ctx); err != nil {
		t.Skipf("integration test skipped: kafka not available (%v); run: make kafka-up", err)
	}

	return brokers
}

func produceTestJob(t *testing.T, brokers []string, topic, eventID string, job engine.Job) {
	t.Helper()

	value, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(topic),
	)
	if err != nil {
		t.Fatalf("producer client: %v", err)
	}
	defer cl.Close()

	res := cl.ProduceSync(ctx, &kgo.Record{
		Key:   []byte(eventID),
		Value: value,
	})
	if err := res.FirstErr(); err != nil {
		t.Fatalf("produce: %v", err)
	}
}

// pollUntilEventID runs the worker consume loop until wantEventID is handled and
// committed, or ctx expires.
func pollUntilEventID(
	ctx context.Context,
	t *testing.T,
	client *kgo.Client,
	logger *slog.Logger,
	wantEventID string,
) {
	t.Helper()

	for {
		fetches := client.PollFetches(ctx)

		if ctx.Err() != nil {
			t.Fatalf("timeout waiting for event %q", wantEventID)
		}

		var found bool

		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			if p.Err != nil {
				t.Fatalf("partition fetch: %v", p.Err)
			}

			var committed []*kgo.Record
			for _, rec := range p.Records {
				var job engine.Job
				if err := json.Unmarshal(rec.Value, &job); err != nil {
					continue
				}
				if job.StripeEventID != wantEventID {
					continue
				}
				if !handleRecord(ctx, logger, &fakeCompletionStore{}, "stripe-webhook-worker", rec) {
					t.Fatalf("handleRecord failed for %q", wantEventID)
				}
				committed = append(committed, rec)
				found = true
			}

			if len(committed) > 0 {
				if err := client.CommitRecords(ctx, committed...); err != nil {
					t.Fatalf("commit: %v", err)
				}
			}
		})

		client.AllowRebalance()

		if found {
			return
		}
	}
}

// sawEventIDAgain returns true if wantEventID appears in any fetched record.
func sawEventIDAgain(ctx context.Context, client *kgo.Client, wantEventID string) bool {
	for {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			return false
		}

		var replayed bool
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			for _, rec := range p.Records {
				var job engine.Job
				if err := json.Unmarshal(rec.Value, &job); err != nil {
					continue
				}
				if job.StripeEventID == wantEventID {
					replayed = true
				}
			}
		})
		client.AllowRebalance()

		if replayed {
			return true
		}
	}
}

func TestWorker_ConsumeJob_Integration(t *testing.T) {
	// Do not t.Parallel — shares local broker/topic.
	brokers := testKafkaBrokers(t)

	topic := strings.TrimSpace(os.Getenv("KAFKA_TOPIC"))
	if topic == "" {
		topic = "stripe-events"
	}

	const eventID = "evt_integration_consume"
	groupID := "stripe-webhook-worker-integration-" + strings.ReplaceAll(t.Name(), "/", "-")

	job := engine.Job{
		StripeEventID: eventID,
		EventType:     "invoice.payment_succeeded",
		Payload:       json.RawMessage(`{"id":"in_integration"}`),
	}
	produceTestJob(t, brokers, topic, eventID, job)

	// --- first consumer: should see our job ---
	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.ClientID("stripe-webhook-worker-test"),
		kgo.WithContext(ctx),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
	)
	if err != nil {
		t.Fatalf("consumer client: %v", err)
	}
	defer client.CloseAllowingRebalance()

	pollUntilEventID(ctx, t, client, logger, eventID)

	fields := lastLogFields(t, &buf)
	if fields["msg"] != stripeJobConsumed {
		t.Fatalf("msg = %v, want %q", fields["msg"], stripeJobConsumed)
	}
	if fields["event_id"] != eventID {
		t.Fatalf("event_id = %v, want %q", fields["event_id"], eventID)
	}

	// --- second consumer, same group: should NOT replay committed offset ---
	client2, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.ClientID("stripe-webhook-worker-test-2"),
		kgo.WithContext(ctx),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
	)
	if err != nil {
		t.Fatalf("consumer client 2: %v", err)
	}
	defer client2.CloseAllowingRebalance()

	replayCtx, replayCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer replayCancel()

	if sawEventIDAgain(replayCtx, client2, eventID) {
		t.Fatalf("event %q replayed after commit", eventID)
	}
}
