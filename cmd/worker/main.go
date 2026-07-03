package main

import (
	"context"
	"encoding/json"
	"integration-engine/internal/config"
	"integration-engine/internal/engine"
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
					if handleRecord(logger, rec) {
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
func handleRecord(logger *slog.Logger, rec *kgo.Record) bool {
	var job engine.Job
	if err := json.Unmarshal(rec.Value, &job); err != nil {
		logger.Error(stripeJobUnmarshalFailed, "error", err.Error())
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
