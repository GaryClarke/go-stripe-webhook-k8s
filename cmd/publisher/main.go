package main

import (
	"context"
	"integration-engine/internal/config"
	"integration-engine/internal/store"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	logger := NewJSONLogger(os.Stdout, nil)

	cfg, err := config.LoadPublisher()
	if err != nil {
		logger.Error(configLoadFailed, "error", err.Error())
		os.Exit(1)
	}

	st, err := store.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		logger.Error(storeOpenFailed, "error", err.Error())
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.KafkaBrokers...),
		kgo.ClientID("stripe-webhook-publisher"),
		kgo.WithContext(ctx),
	)
	if err != nil {
		logger.Error(kafkaClientFailed, "error", err.Error())
		os.Exit(1)
	}
	defer client.Close()

	logger.Info(publisherStarted,
		"brokers", cfg.KafkaBrokers,
		"topic", cfg.KafkaTopic,
		"poll_interval", cfg.PollInterval.String(),
	)

	for {
		if ctx.Err() != nil {
			break
		}

		row, err := st.NextPendingOutbox(ctx)
		if err != nil {
			logger.Error(outboxPublishFailed, "error", err.Error())
			sleepOrDone(ctx, cfg.PollInterval)
			continue
		}
		if row == nil {
			sleepOrDone(ctx, cfg.PollInterval)
			continue
		}

		_, err = publishOutbox(ctx, client, cfg.KafkaTopic, row.EventID, row.Payload)
		if err != nil {
			logger.Error(outboxPublishFailed,
				"event_id", row.EventID,
				"topic", cfg.KafkaTopic,
				"error", err.Error(),
			)
			sleepOrDone(ctx, cfg.PollInterval)
			continue
		}
		updated, err := st.MarkOutboxPublished(ctx, row.EventID)
		if err != nil {
			logger.Error(outboxPublishFailed,
				"event_id", row.EventID,
				"topic", cfg.KafkaTopic,
				"error", err.Error(),
			)
			sleepOrDone(ctx, cfg.PollInterval)
			continue
		}
		if !updated {
			logger.Error(outboxMarkPublishedFailed,
				"event_id", row.EventID,
				"topic", cfg.KafkaTopic,
				"error", "conditional update matched 0 rows",
			)
			sleepOrDone(ctx, cfg.PollInterval)
			continue
		}
		logger.Info(outboxPublishSucceeded,
			"event_id", row.EventID,
			"topic", cfg.KafkaTopic,
		)
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
