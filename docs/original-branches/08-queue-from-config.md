# Branch 08: Queue from config (factory)

**Goal:** Construct the correct queue implementation from loaded configuration—config-driven backend choice for local (`memory`) and a clear placeholder for production (`sqs` until Phase 2). No call site in `cmd/*` required until the ingest path consumes an `Enqueuer`.

## What we're adding

- **NewFromConfig** - Returns `(Enqueuer, Consumer, error)`; for `memory`, the same `*MemoryQueue` is returned for both interfaces
- **ErrSQSNotImplemented** - Returned when `QUEUE_BACKEND=sqs` until SQS is implemented

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| `*config.Config` | Pointer, no nil guard | Matches `config.Load()`; internal callers only—contract is non-nil after successful load. |
| Buffer size | `NewMemory(defaultBufferSize)` | Same package constant as `memory.go`; avoids magic `0` sentinel at the factory call site. |
| Normalisation | `strings.ToLower` / `TrimSpace` on `QueueBackend` | Tolerates casing and stray whitespace from env. |
| Unknown backend | `fmt.Errorf` with quoted value | Easier debugging than silent fallback. |

## Steps (in order)

1. Add `internal/queue/factory.go` with `NewFromConfig` and `ErrSQSNotImplemented`.
2. Run `go build ./...` to verify.

## Files changed/added

- `internal/queue/factory.go` - Factory

## Next branch

**09-parse-stripe-event** - `ParseStripeEvent` decodes raw JSON to `StripeEvent`; tests use `testdata`. See [09-parse-stripe-event.md](09-parse-stripe-event.md). Later branches map to `Job`, `Enqueue`, and HTTP/Lambda wiring per `docs/PROJECT_KNOWLEDGE.md` §5.
