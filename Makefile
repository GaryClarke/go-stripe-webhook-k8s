# Integration Engine - common tasks
# See docs/PROJECT_KNOWLEDGE.md for project overview.

.PHONY: build test lint tidy build-ingest fmt

build:
	go build ./...

# Build Lambda zip for ingest (payments-events-ingest). Output: build/bootstrap.zip
build-ingest:
	@mkdir -p build
	GOOS=linux GOARCH=amd64 go build -o build/bootstrap ./cmd/ingest
	cd build && zip -q -o bootstrap.zip bootstrap

test:
	go test -v ./...

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || true

tidy:
	go mod tidy

fmt:
	terraform fmt -recursive terraform/
