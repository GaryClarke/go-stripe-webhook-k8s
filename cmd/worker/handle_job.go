// cmd/worker/handle_job.go
package main

import (
	"context"
	"integration-engine/internal/engine"
	"log/slog"
)

func handleJob(ctx context.Context, logger *slog.Logger, job engine.Job) error {
	logger.Info(stripeJobHandled,
		"event_id", job.StripeEventID,
		"event_type", job.EventType,
	)
	return nil
}
