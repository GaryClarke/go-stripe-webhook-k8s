package main

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

type kafkaProducer interface {
	ProduceSync(ctx context.Context, rs ...*kgo.Record) kgo.ProduceResults
}

func publishOutbox(ctx context.Context, producer kafkaProducer, topic string, eventID string, payload []byte) (*kgo.Record, error) {
	rec := &kgo.Record{
		Topic: topic,
		Key:   []byte(eventID),
		Value: payload,
	}
	results := producer.ProduceSync(ctx, rec)
	if err := results.FirstErr(); err != nil {
		return nil, err
	}
	return rec, nil
}
