-- +goose Up
-- M10: consumer completion ledger — one row per (event_id, consumer_name).
-- Answers: "Did this consumer finish downstream work?" Separate from ingestion (accepted) and outbox (published).

CREATE TABLE consumer_completions (
    event_id        TEXT NOT NULL,
    consumer_name   TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    status          TEXT NOT NULL,
    error           TEXT,
    attempt_count   INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at    TIMESTAMPTZ,
    PRIMARY KEY (event_id, consumer_name),
    CONSTRAINT consumer_completions_status_check
      CHECK (status IN ('processing', 'processed', 'failed'))
);

CREATE INDEX consumer_completions_status_idx ON consumer_completions (status);

-- +goose Down
DROP INDEX IF EXISTS consumer_completions_status_idx;
DROP TABLE IF EXISTS consumer_completions;