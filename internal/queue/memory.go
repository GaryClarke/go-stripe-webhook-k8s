package queue

import (
	"context"
	"errors"

	"integration-engine/internal/engine"
)

const defaultBufferSize = 1024

// MemoryQueue is a buffered in-process queue for local development.
type MemoryQueue struct {
	ch chan *engine.Job
}

// NewMemory creates an in-memory queue with a fixed buffer size.
// Enqueue blocks when the buffer is full; Consume blocks when empty.
func NewMemory(bufferSize int) *MemoryQueue {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}
	return &MemoryQueue{ch: make(chan *engine.Job, bufferSize)}
}

// Enqueue adds job to the queue. It blocks until there is buffer space or ctx is
// cancelled. Returns ctx.Err() if the context ends before the send completes.
// A nil job returns an error without enqueuing.
func (q *MemoryQueue) Enqueue(ctx context.Context, job *engine.Job) error {
	if job == nil {
		return errors.New("nil job")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.ch <- job:
		return nil
	}
}

// Consume removes and returns the next job. It blocks until a job is available or ctx
// is cancelled. On cancellation, returns (nil, ctx.Err()).
func (q *MemoryQueue) Consume(ctx context.Context) (*engine.Job, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case job := <-q.ch:
		return job, nil
	}
}

// Compile-time checks: *MemoryQueue must implement Enqueuer and Consumer. If methods
// drift, the build breaks here. Optional—not required by Go—just a useful guard.
var (
	_ Enqueuer = (*MemoryQueue)(nil)
	_ Consumer = (*MemoryQueue)(nil)
)
