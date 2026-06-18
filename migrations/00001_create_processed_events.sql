-- +goose Up
-- Event processing ledger: one row per Stripe event.id (M8 idempotency).
-- status lifecycle: processing -> processed (failed reserved for later recovery).
CREATE TABLE processed_events (
    event_id       TEXT PRIMARY KEY,
    event_type     TEXT NOT NULL,
    status         TEXT NOT NULL,
    received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at   TIMESTAMPTZ,
    failed_at      TIMESTAMPTZ,
    error_message  TEXT,
    CONSTRAINT processed_events_status_check
      CHECK (status IN ('processing', 'processed', 'failed'))
);

-- +goose Down
DROP TABLE IF EXISTS processed_events;