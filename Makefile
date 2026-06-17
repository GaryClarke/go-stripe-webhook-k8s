# Integration Engine - common tasks
# See docs/PROJECT_KNOWLEDGE.md for project overview.

.PHONY: build test lint tidy fmt lab-status lab-ecr-refresh lab-on lab-off

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

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || true

tidy:
	go mod tidy

fmt:
	terraform fmt -recursive terraform/

.PHONY: build test lint tidy fmt lab-status lab-ecr-refresh lab-on lab-off \
	db-up db-down db-logs db-check db-migrate db-migrate-test

# --- Local Postgres (M8 Phase 1) — see docs/branches/16-idempotency-postgres.md ---
# Dev and test are separate DATABASE names on the same Compose service (port 5433).

DATABASE_URL_DEV ?= postgres://webhook:webhook@localhost:5433/stripe_webhook_dev?sslmode=disable
DATABASE_URL_TEST ?= postgres://webhook:webhook@localhost:5433/stripe_webhook_test?sslmode=disable

# Start local Postgres (detached). Reuses existing volume unless you ran db-down with -v.
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

# Phase 2: Goose migrations (targets wired now; goose + migrations/ come next).
db-migrate:
	@echo "Phase 2: install goose and add migrations/ — then:"
	@echo "  goose -dir migrations postgres \"$(DATABASE_URL_DEV)\" up"
	@exit 1

db-migrate-test:
	@echo "Phase 2: install goose and add migrations/ — then:"
	@echo "  goose -dir migrations postgres \"$(DATABASE_URL_TEST)\" up"
	@exit 1
