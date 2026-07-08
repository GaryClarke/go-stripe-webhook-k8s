-- +goose Up
-- M9b: ledger status "accepted" (ingestion done) + transactional outbox queue.

-- Extend ledger CHECK; migrate M8 "processed" rows to "accepted" (ingested, not worker-done).
ALTER TABLE processed_events DROP CONSTRAINT processed_events_status_check;
ALTER TABLE processed_events ADD CONSTRAINT processed_events_status_check
    CHECK (status IN ('processing', 'accepted', 'processed', 'failed'));
UPDATE processed_events SET status = 'accepted' WHERE status = 'processed';

-- Outbox: one row per accepted event (1:1 with event_id for M9).
CREATE TABLE outbox_events (
    id             BIGSERIAL PRIMARY KEY,
    event_id       TEXT NOT NULL UNIQUE REFERENCES processed_events(event_id),
    event_type     TEXT NOT NULL,
    payload        JSONB NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at   TIMESTAMPTZ,
    CONSTRAINT outbox_events_status_check
       CHECK (status IN ('pending', 'published', 'failed'))
);

-- +goose Down
DROP TABLE IF EXISTS outbox_events;

UPDATE processed_events SET status = 'processed' WHERE status = 'accepted';

ALTER TABLE processed_events DROP CONSTRAINT processed_events_status_check;
ALTER TABLE processed_events ADD CONSTRAINT processed_events_status_check
    CHECK (status IN ('processing', 'processed', 'failed'));