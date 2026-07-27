// cmd/worker/handle_job.go
package main

import (
	"context"
	"errors"
	"integration-engine/internal/engine"
	"log/slog"
)

func handleJob(
	ctx context.Context,
	logger *slog.Logger,
	downstream DownstreamClient,
	job engine.Job,
) error {
	if err := downstream.DeliverJob(ctx, job); err != nil {
		retryable := true
		httpStatus := 0
		if je, ok := errors.AsType[*JobError](err); ok {
			retryable = je.Retryable
			httpStatus = je.HTTPStatus
		}

		logger.Error(stripeJobDownstreamFailed,
			"event_id", job.StripeEventID,
			"event_type", job.EventType,
			"retryable", retryable,
			"http_status", httpStatus,
			"error", err.Error(),
		)
		return err
	}

	logger.Info(stripeJobHandled,
		"event_id", job.StripeEventID,
		"event_type", job.EventType,
	)
	return nil
}
