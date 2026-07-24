# Integration Engine - common tasks
# See docs/PROJECT_KNOWLEDGE.md for project overview.

# ROSA lab (gc-rosa-lab) — see docs/rosa/runbook.md and cursor-rules.md "start work"
lab-status:
	./scripts/lab-status.sh

lab-ecr-refresh:
	./scripts/rosa-lab-ecr-secret.sh

lab-on:
	./scripts/rosa-lab-on.sh

lab-off:
	./scripts/rosa-lab-off.sh

build:
	go build ./...

test:
	go test -v ./...

# Kafka + Postgres worker integration tests (requires make kafka-up for ConsumeJob; db-up + db-migrate-test for completion tests).
test-integration:
	go test -tags=integration ./cmd/worker/... -v

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || true

tidy:
	go mod tidy

fmt:
	terraform fmt -recursive terraform/

.PHONY: build test test-integration lint tidy fmt lab-status lab-ecr-refresh lab-on lab-off \
	db-up db-down db-logs db-check db-migrate db-migrate-test \
	kafka-up kafka-down kafka-logs kafka-check kafka-smoke worker-run publisher-run

# --- Local Postgres (M8 Phase 1) — see docs/branches/16-idempotency-postgres.md ---
# Dev and test are separate DATABASE names on the same Compose service (port 5433).

DATABASE_URL_DEV ?= postgres://webhook:webhook@localhost:5433/stripe_webhook_dev?sslmode=disable
DATABASE_URL_TEST ?= postgres://webhook:webhook@localhost:5433/stripe_webhook_test?sslmode=disable
# Publisher (and manual go run) default to dev DB when DATABASE_URL is unset.
DATABASE_URL ?= $(DATABASE_URL_DEV)

# Start local Compose stack (Postgres + Redpanda + Console). Reuses volumes unless db-down -v.
db-up:
	docker compose up -d

# Stop containers. Add: docker compose down -v  to wipe local DB data (re-runs init scripts).
db-down:
	docker compose down

# Tail Postgres logs (Ctrl+C to exit).
db-logs:
	docker compose logs -f db

# Smoke check: container healthy and both databases exist.
db-check:
	docker compose ps
	docker compose exec db psql -U webhook -d stripe_webhook_dev -c '\l'

# Goose runs SQL migrations in migrations/ (see docs/branches/16-idempotency-postgres.md).
GOOSE := go tool goose

db-migrate:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL_DEV)" up

db-migrate-test:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL_TEST)" up

# --- Local Redpanda (M9a Phase 1) — see docs/branches/17-kafka-outbox.md ---
# Kafka-compatible broker for local dev. Go clients on the host use localhost:19092 (not 9092).

KAFKA_BROKERS ?= localhost:19092
KAFKA_TOPIC ?= stripe-events
KAFKA_GROUP_ID ?= stripe-webhook-worker
# Worker downstream HTTP (M11). Override when a mock or real endpoint is listening.
DOWNSTREAM_URL ?= http://localhost:8080/downstream

# Start broker + Console only (Postgres can stay stopped).
kafka-up:
	docker compose up -d redpanda console

kafka-down:
	docker compose stop redpanda console

kafka-logs:
	docker compose logs -f redpanda console

# Broker healthy + cluster metadata.
kafka-check:
	docker compose ps redpanda console
	docker compose exec redpanda rpk cluster info

# Produce one test record and consume it (Phase 1 done gate).
kafka-smoke:
	printf '%s\n' '{"stripe_event_id":"evt_smoke"}' | docker compose exec -T redpanda rpk topic produce $(KAFKA_TOPIC) -k evt_smoke
	docker compose exec redpanda rpk topic consume $(KAFKA_TOPIC) -o -1 -n 1

worker-run:
	DATABASE_URL=$(DATABASE_URL) \
	DOWNSTREAM_URL=$(DOWNSTREAM_URL) \
	KAFKA_BROKERS=$(KAFKA_BROKERS) KAFKA_TOPIC=$(KAFKA_TOPIC) KAFKA_GROUP_ID=$(KAFKA_GROUP_ID) \
	go run ./cmd/worker

publisher-run:
	DATABASE_URL=$(DATABASE_URL) \
	KAFKA_BROKERS=$(KAFKA_BROKERS) KAFKA_TOPIC=$(KAFKA_TOPIC) \
	go run ./cmd/publisher
