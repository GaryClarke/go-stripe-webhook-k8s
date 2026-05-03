package queue

import (
	"errors"
	"fmt"
	"strings"

	"integration-engine/internal/config"
)

// ErrSQSNotImplemented is returned when QUEUE_BACKEND=sqs until Phase 2 adds SQS.
var ErrSQSNotImplemented = errors.New("queue: SQS backend not implemented")

// NewFromConfig builds the queue implementation for this process.
// For QUEUE_BACKEND=memory, the same *MemoryQueue is returned as both Enqueuer and Consumer.
func NewFromConfig(cfg *config.Config) (Enqueuer, Consumer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.QueueBackend)) {
	case "memory", "":
		m := NewMemory(defaultBufferSize)
		return m, m, nil
	case "sqs":
		return nil, nil, ErrSQSNotImplemented
	default:
		return nil, nil, fmt.Errorf("queue: unknown QUEUE_BACKEND %q", cfg.QueueBackend)
	}
}
