package queue

import (
	"context"

	"integration-engine/internal/engine"
)

// Enqueuer publishes jobs for the worker to process (ingest side).
type Enqueuer interface {
	Enqueue(ctx context.Context, job *engine.Job) error
}

// Consumer consumes jobs from the queue (worker side).
type Consumer interface {
	Consume(ctx context.Context) (*engine.Job, error)
}
