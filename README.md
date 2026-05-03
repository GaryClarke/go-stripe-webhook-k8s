# Integration Engine

Event-driven webhook integration engine: receive → validate → queue → process → forward. Built in Go, deployable to AWS (Lambda, SQS, API Gateway).

## Docs

- **[Project knowledge](docs/PROJECT_KNOWLEDGE.md)** - Plan, local vs Lambda, how Lambda is invoked, build order.
- **Branch logs** - `docs/branches/` (decisions and scope per branch).

## Quick start (after Phase 1)

```bash
# Dependencies
go mod tidy

# Lint and test
make lint
make test

# Local run (when entry points exist)
# make run-local
```

## Layout

- `cmd/` - Entry points (ingest-local, ingest Lambda, worker-local, worker Lambda).
- `internal/` - Shared packages (engine, queue, config, handlers).
- `testdata/` - Fixtures (e.g. Stripe webhook samples).
- `docs/` - Project knowledge and branch documentation.

See [docs/PROJECT_KNOWLEDGE.md](docs/PROJECT_KNOWLEDGE.md) for architecture and phases.
