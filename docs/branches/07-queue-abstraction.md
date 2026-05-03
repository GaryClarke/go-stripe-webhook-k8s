# Branch 07: Queue abstraction (interfaces + in-memory)

**Goal:** Define queue contracts for ingest vs worker and ship a Phase 1 in-memory implementation. No config wiring yet—callers use `NewMemory` directly until branch 08.

## What we're adding

- **Enqueuer** - `Enqueue(ctx, *engine.Job) error` (ingest side)
- **Consumer** - `Consume(ctx) (*engine.Job, error)` (worker side)
- **MemoryQueue** - Buffered channel-backed queue; context-aware send/receive so a full or empty buffer does not block past cancellation/deadlines
- **NewMemory** - Exported constructor; non-positive buffer size uses an internal default

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| Two interfaces | `Enqueuer` + `Consumer` | Matches ingest-only send vs worker-only receive; same concrete type implements both in-memory. |
| Method name | `Consume` on `Consumer` | Aligns type name; alternative was `Receive`. |
| Exported type | `*MemoryQueue` | Avoids "exported func, unexported return type" lint from `NewMemory`. |
| Blocking + `context` | `select` on `ctx.Done()` and channel op | Standard pattern when enqueue/consume might wait and the caller can time out or shut down. |
| Interface checks | `var _ Enqueuer = (*MemoryQueue)(nil)` (and `Consumer`) | Optional compile-time check that methods still satisfy interfaces. |
| Nil jobs | Reject `nil` in `Enqueue` | Distinguishes invalid enqueue from zero-value `Job{}`. |

## Steps (in order)

1. Add `internal/queue/queue.go` with `Enqueuer` and `Consumer`.
2. Add `internal/queue/memory.go` with `MemoryQueue`, `NewMemory`, and compile-time checks if desired.
3. Run `go build ./...` to verify.

## Files changed/added

- `internal/queue/queue.go` - Interfaces
- `internal/queue/memory.go` - `MemoryQueue` implementation

## Next branch

**08-queue-from-config** - Factory that builds the queue from `config.Config` (`memory` vs `sqs` stub). See [08-queue-from-config.md](08-queue-from-config.md).
