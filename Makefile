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
