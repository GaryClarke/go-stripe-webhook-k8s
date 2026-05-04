# Branch 06: Config (env + godotenv)

**Goal:** Centralise environment-based configuration, validate what Phase 2 will need, and load optional `.env` for local dev. Ingest Lambda fails fast at startup if config is invalid.

## What we're adding

- **Config** - Struct holding `QUEUE_BACKEND`, `SQS_QUEUE_URL`, `AWS_REGION`, `STRIPE_WEBHOOK_SECRET`
- **Load()** - Reads `os.Getenv` after `godotenv.Load()` (missing `.env` is ignored)
- **Validation** - When `QUEUE_BACKEND=sqs`, require `SQS_QUEUE_URL` and `AWS_REGION`
- **Ingest bootstrap** - `cmd/ingest` calls `config.Load()` before `lambda.Start`

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| godotenv | `Load()` error ignored | If no `.env`, use OS/env only (Terraform on Lambda, CI exports). |
| Default queue | `QUEUE_BACKEND` empty → `memory` | Matches Phase 1 local behaviour. |
| Stripe secret | Loaded but not validated yet | Signature verification comes with ingest handler; no hard failure in healthz-only path. |
| Dependency | `github.com/joho/godotenv` via `go get` | Standard helper; version pinned in `go.mod` / `go.sum`. |

## Steps (in order)

1. Add `internal/config` with `Config` and `Load()`.
2. Run `go get github.com/joho/godotenv`.
3. Wire `config.Load()` in `cmd/ingest/main.go` with `log.Fatalf` on error.
4. Run `go build ./...` to verify.

## Files changed/added

- `internal/config/config.go` - Config struct, Load, validation for `sqs`
- `cmd/ingest/main.go` - Load config at startup
- `go.mod` / `go.sum` - godotenv dependency

## Next branch

**07-queue-abstraction** - `Enqueuer` / `Consumer` interfaces and in-memory `MemoryQueue`. See [07-queue-abstraction.md](07-queue-abstraction.md).

**08-queue-from-config** - `NewFromConfig` and `QUEUE_BACKEND` wiring. See [08-queue-from-config.md](08-queue-from-config.md).
