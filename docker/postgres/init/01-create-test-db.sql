-- Runs once when the Postgres data volume is first created (empty volume only).
-- Creates a separate database for integration tests so tests can truncate/reset
-- without wiping stripe_webhook_dev data you use for manual dev.
--
-- Re-runs only after: docker compose down -v  (destroys local DB data)

CREATE DATABASE stripe_webhook_test;