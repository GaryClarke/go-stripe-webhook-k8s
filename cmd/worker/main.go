package main

import (
	"context"
	"encoding/json"
	"integration-engine/internal/config"
	"integration-engine/internal/engine"
	"integration-engine/internal/store"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	logger := NewJSONLogger(os.Stdout, nil)

	cfg, err := config.LoadWorker()
	if err != nil {
		logger.Error(configLoadFailed, "error", err.Error())
		os.Exit(1)
	}
	st, err := store.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		logger.Error(storeOpenFailed, "error", err.Error())
		os.Exit(1)
	}
	downstream := NewHTTPDownstream(cfg.DownstreamURL, nil)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.KafkaBrokers...),
		kgo.ConsumerGroup(cfg.KafkaGroupID),
		kgo.ConsumeTopics(cfg.KafkaTopic),
		kgo.ClientID("stripe-webhook-worker"),
		kgo.WithContext(ctx),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
	)
	if err != nil {
		logger.Error(kafkaClientFailed, "error", err.Error())
		os.Exit(1)
	}
	defer client.CloseAllowingRebalance()

	logger.Info(workerStarted,
		"brokers", cfg.KafkaBrokers,
		"topic", cfg.KafkaTopic,
		"group_id", cfg.KafkaGroupID,
	)

	for {
		fetches := client.PollFetches(ctx)

		if ctx.Err() == nil {
			for _, fetchErr := range fetches.Errors() {
				logger.Error(kafkaFetchFailed,
					"error", fetchErr.Err.Error(),
					"topic", fetchErr.Topic,
					"partition", fetchErr.Partition,
				)
			}

			fetches.EachPartition(func(p kgo.FetchTopicPartition) {
				if p.Err != nil {
					logger.Error(kafkaFetchFailed,
						"error", p.Err.Error(),
						"topic", p.Topic,
						"partition", p.Partition,
					)
					return
				}

				var committed []*kgo.Record
				for _, rec := range p.Records {
					if handleRecord(ctx, logger, st, downstream, cfg.KafkaGroupID, rec) {
						committed = append(committed, rec)
					}
				}

				if len(committed) > 0 {
					if err := client.CommitRecords(ctx, committed...); err != nil {
						logger.Error(kafkaCommitFailed, "error", err.Error())
					}
				}
			})
		}

		client.AllowRebalance()

		if ctx.Err() != nil {
			break
		}
	}

	logger.Info(shuttingDown)
}

// handleRecord returns true if the record was handled successfully (safe to commit).
func handleRecord(
	ctx context.Context,
	logger *slog.Logger,
	st store.ConsumerCompletionStore,
	downstream DownstreamClient,
	consumerName string,
	rec *kgo.Record,
) bool {
	// Step 1: empty struct
	var job engine.Job
	// Step 2: fill it from Kafka message bytes
	if err := json.Unmarshal(rec.Value, &job); err != nil {
		logger.Error(stripeJobUnmarshalFailed, "error", err.Error())
		return false
	}
	// ↑ After this line, job.StripeEventID and job.EventType are set
	//   (if they were in the JSON — publisher put them there when AcceptEvent ran)
	if job.StripeEventID == "" {
		logger.Error(stripeJobUnmarshalFailed, "error", "missing stripe_event_id")
		return false
	}

	// Step 3: NOW you can claim — job exists and has fields
	claim, err := st.ClaimConsumerCompletion(ctx, job.StripeEventID, consumerName, job.EventType)
	if err != nil {
		logger.Error(consumerCompletionFailed, "event_id", job.StripeEventID, "error", err.Error())
		return false
	}

	if claim.Action == store.CompletionClaimAlreadyProcessed {
		logger.Info(stripeJobDuplicateSkipped,
			"event_id", job.StripeEventID,
			"event_type", job.EventType,
			"partition", rec.Partition,
			"offset", rec.Offset,
		)
		return true
	}
	// new / retry / retry_from_failed → handleJob → mark → return true

	if err := handleJob(ctx, logger, downstream, job); err != nil {
		if _, markErr := st.MarkConsumerFailed(ctx, job.StripeEventID, consumerName, err.Error()); markErr != nil {
			logger.Error(consumerCompletionFailed, "event_id", job.StripeEventID, "error", markErr.Error())
			return false
		}
		return !isRetryableJobError(err)
	}

	updated, err := st.MarkConsumerProcessed(ctx, job.StripeEventID, consumerName)
	if err != nil {
		logger.Error(consumerCompletionFailed, "event_id", job.StripeEventID, "error", err.Error())
		return false
	}
	if !updated {
		logger.Error(consumerCompletionFailed, "event_id", job.StripeEventID, "error", "mark processed: row not processing")
		return false
	}

	logger.Info(stripeJobConsumed,
		"event_id", job.StripeEventID,
		"event_type", job.EventType,
		"partition", rec.Partition,
		"offset", rec.Offset,
	)
	return true
}
