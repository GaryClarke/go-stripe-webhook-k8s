# Branch 18: Consumer completion (`18-consumer-completion`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 10** — record **downstream processing outcome** in a dedicated table, make the worker **idempotent** under Kafka at-least-once delivery, and **commit offsets only after** a durable completion record.

**Git branch:** `18-consumer-completion`

**Status:** **Complete (local)** — store, worker, E2E verified on branch **`18-consumer-completion`**.

**Prerequisite:** **Milestone 9** complete on **`main`** (outbox, publisher, worker consume + log).

**Cloud:** **Out of scope** — local Compose only (Postgres + Redpanda). No ROSA, AWS, MSK, or K8s deploy of worker in this branch.

---

## Decisions (locked for M10)

| Decision | Choice |
|----------|--------|
| **Third table** | **`consumer_completions`** — do **not** extend **`processed_events`** or **`outbox_events`** lifecycles |
| **Primary key** | **`(event_id, consumer_name)`** — supports multiple consumers later |
| **`consumer_name`** | Default from **`KAFKA_GROUP_ID`** (e.g. **`stripe-webhook-worker`**) — same value worker already uses |
| **Statuses** | **`processing`**, **`processed`**, **`failed`** (CHECK constraint) |
| **Idempotency** | If row already **`processed`** for this pair → log **`stripe_job_duplicate_skipped`**, return success (safe to commit offset) |
| **Offset commit** | **`CommitRecords` only after** completion row is durably written (or duplicate skip confirmed) |
| **Ingestion ledger** | **`processed_events.status`** stays **`accepted`** — unchanged |
| **Outbox** | **`outbox_events.status`** stays **`published`** — unchanged |
| **`processed` meaning** | Allowed on **consumer** table only — means *this consumer finished*, not *Stripe pipeline end-to-end* |
| **Downstream IO (M10)** | **Stub** — e.g. log **`stripe_job_handled`** or no-op success; real HTTP to third party deferred to M11+ |
| **Worker DB** | Worker needs **`DATABASE_URL`** (same dev Postgres as API/publisher) |
| **Failed handling** | Persist **`failed`** + sanitized **`error`**; **do not** commit offset (Kafka redelivers) — M10 minimum |
| **`processing` in-flight** | **Retry** — if row already **`processing`** (crash mid-handle), proceed with handle again; bump **`attempt_count`** on reclaim; no stale skip in M10 |
| **`failed` on reclaim** | **Retry from failed** — reset to **`processing`**, bump **`attempt_count`**, clear **`error`** → action **`retry_from_failed`** |
| **`ClaimConsumerCompletion` return** | **`*CompletionClaim`** with **`CompletionClaimAction`** enum (`new`, `retry`, `retry_from_failed`, `already_processed`) |
| **`CompletionStatus`** | On **`Store`** for **integration tests**; **not** used on worker hot path (claim + mark are enough) |
| **`store.go` layout** | Group by **domain** (ledger consts + types, outbox, completion, then interfaces) — not strict “all consts first” |
| **Never log** | **`DATABASE_URL`**, raw **`payload`**, secrets |

**Why a third table:** **`processed_events`** answers “accepted from Stripe?”; **`outbox_events`** answers “published to Kafka?”; **`consumer_completions`** answers “did consumer X finish?” Mixing those stages into one row does not scale when multiple consumers exist.

---

## Architecture (target)

```text
Layer summary (after M10):

  processed_events      = "Did we accept this event_id from Stripe?"     -> accepted
  outbox_events         = "Did we publish to Kafka?"                    -> published
  consumer_completions  = "Did consumer X process it?"                  -> processed | failed
```

```text
cmd/worker (per message)
  -> unmarshal engine.Job
  -> ClaimConsumerCompletion (INSERT processing ON CONFLICT ...)
       if already processed: log stripe_job_duplicate_skipped -> commit offset
       if processing (in-flight): retry handle (M10 — bump attempt_count, no skip)
  -> handleJob (M10: stub — log stripe_job_handled)
  -> MarkCompletionProcessed (UPDATE ... WHERE status = processing)
       on error: MarkCompletionFailed; return false (no offset commit)
  -> log stripe_job_consumed (keep) or fold into stripe_job_handled
  -> CommitRecords
```

**Crash windows:**

| Order | Crash after | On redelivery |
|-------|-------------|---------------|
| DB **`processed`** then commit offset | offset not committed | Duplicate → skip via completion row |
| commit offset before DB | offset committed, no row | **Bad** — avoid (M10 rule) |

---

## Schema

Goose e.g. **`migrations/00003_consumer_completions.sql`**:

```sql
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
```

**FK to `processed_events(event_id)`:** optional in M10 (simpler without FK first); add later if desired.

---

## Phases

### Phase 0 — Lock model (this doc)

**Done gate:** Four-layer diagram understood; third table rationale clear.

**Locked:** **`processing`** in-flight → **retry** (not skip); see Decisions table.

---

### Phase 1 — Migration + store

**Do:**

1. Migration **`00003_consumer_completions.sql`**
2. Store constants + **`ConsumerCompletionRow`**
3. **`ClaimConsumerCompletion(ctx, eventID, consumerName, eventType)`** — insert **`processing`**; on conflict return existing status; if existing **`processing`**, increment **`attempt_count`** and allow retry (return claimable)
4. **`MarkConsumerProcessed(ctx, eventID, consumerName)`** — conditional update from **`processing`**
5. **`MarkConsumerFailed(ctx, eventID, consumerName, errMsg)`** — conditional update from **`processing`**
6. Integration tests: claim → processed; duplicate claim → already processed; failed path

**Done gate:** **`go test ./internal/store/...`** green with Postgres test DB.

---

### Phase 1b — Store interface segregation (after Phase 1 PR)

**When:** Right after Phase 1 store methods + integration tests merge (or commit on same branch before Phase 2). **No SQL changes** — refactor types and wiring only.

**Why:** **`Store`** is a wide facade (~11 methods). **`fakeStore`** in **`cmd/api`** must stub outbox and completion methods the API never calls. **`cmd/publisher`** already uses **`*store.Postgres`** directly, not **`Store`**.

**Goal:** Interface segregation at **call sites**; **`*Postgres`** still one struct implementing everything.

**Do:**

1. Split **`internal/store/store.go`** into embedded interfaces:

| Interface | Methods | Primary consumer |
|-----------|---------|------------------|
| **`LedgerStore`** | **`AcceptEvent`**, **`Status`** | **`cmd/api`** |
| **`LegacyLedgerStore`** (optional) | **`ProcessEvent`** | M8 tests only — drop from composite when unused |
| **`OutboxStore`** | **`NextPendingOutbox`**, **`MarkOutboxPublished`** | **`cmd/publisher`** |
| **`OutboxReader`** (optional) | **`OutboxStatus`** | integration tests |
| **`ConsumerCompletionStore`** | **`ClaimConsumerCompletion`**, **`MarkConsumerProcessed`**, **`MarkConsumerFailed`** | **`cmd/worker`** |
| **`CompletionReader`** (optional) | **`CompletionStatus`** | integration tests |
| **`Pinger`** | **`Ping`** | **`cmd/api`** **`/readyz`** |

2. **Composite facade** (for **`main`** wiring and full integration tests):

```go
type Store interface {
    LedgerStore
    OutboxStore
    ConsumerCompletionStore
    Pinger
    // optional: OutboxReader, CompletionReader, LegacyLedgerStore
}
```

3. **Narrow dependencies at call sites:**

| Package | Depend on |
|---------|-----------|
| **`cmd/api/app.go`** | **`LedgerStore` + `Pinger`** (local interface embedding those two, or named **`IngestStore`**) |
| **`cmd/publisher`** | **`OutboxStore`** via interface (replace raw **`*Postgres`** type if desired) |
| **`cmd/worker`** | **`ConsumerCompletionStore`** only (Phase 2) |

4. **Shrink **`fakeStore`**** — implement only **`LedgerStore` + `Pinger`** for API unit tests; use **`store.Postgres`** or a test helper for cross-layer integration tests.

5. **File layout in **`store.go`** (or split **`store/interfaces.go`** if file grows): group by domain — ledger consts + **`EventStatus`**, outbox consts + outbox types, completion consts + **`CompletionClaim`** + **`CompletionStatus`**, then interface blocks.

**Done gate:** **`go test ./...`** green; **`fakeStore`** no longer stubs completion/outbox publish methods unless a test needs them.

**Out of scope for 1b:** new behaviour, migration changes, worker wiring (that is Phase 2).

---

### Phase 2 — Worker config + wiring

**Do:**

1. **`LoadWorker()`** — add required **`DATABASE_URL`**
2. Open Postgres in **`cmd/worker/main.go`**
3. Inject store into **`handleRecord`** (or small **`handler` struct**)
4. **`Makefile`** **`worker-run`** — pass **`DATABASE_URL`** (mirror **`publisher-run`**)

**Done gate:** Worker starts with DB; fails fast without **`DATABASE_URL`**.

---

### Phase 3 — Idempotent handle + offset discipline

**Do:**

1. **`ClaimConsumerCompletion`** before business logic
2. If already **`processed`** → **`stripe_job_duplicate_skipped`**, return **`true`**
3. Stub **`handleJob`** (log only)
4. **`MarkConsumerProcessed`** on success → return **`true`**
5. On mark/handle error → **`MarkConsumerFailed`** where appropriate, return **`false`** (no commit)

**Done gate:** Manual E2E — completion row **`processed`**; Console lag **0**; second consumer group or offset reset shows duplicate skip (optional test).

---

### Phase 4 — Tests + README

**Do:**

1. Unit tests for handle path (fake store)
2. Optional integration test: produce Job → worker → row in **`consumer_completions`**
3. README: fourth layer in E2E section; **`worker-run`** needs **`DATABASE_URL`**
4. Update **`PLAN.md`** M10 status when done gate met

**Done gate:** Full local path documented:

```text
stripe listen --latest -> API -> outbox -> publisher -> Kafka -> worker -> consumer_completions
```

---

## Log messages (M10)

| Message | When |
|---------|------|
| **`stripe_job_consumed`** | Keep or merge — message seen and past claim |
| **`stripe_job_duplicate_skipped`** | Completion already **`processed`** |
| **`stripe_job_handled`** | Stub downstream success (optional) |
| **`consumer_completion_failed`** | Mark failed or claim error (sanitized) |

---

## Verify checklist

- [x] Phase 0: Third table + offset-after-DB understood; **`processing`** → retry locked
- [x] Phase 1: Migration + store methods + integration tests
- [ ] Phase 1b: Store interface segregation (optional refactor, no SQL) — partial: **`ConsumerCompletionStore`** for worker
- [x] Phase 2: Worker **`DATABASE_URL`** + store wired
- [x] Phase 3: Idempotent handle + commit discipline
- [x] Phase 4: README + PLAN M10 complete

---

## Out of scope (M10)

- ROSA / K8s worker Deployment
- MSK / managed Kafka / TLS
- DLQ topic
- Real downstream HTTP client
- Multiple consumer types (schema supports it; only one worker in M10)
- Outbox **`failed`** retry / multi-publisher claims (post-M10 local hardening)

---

## Follow-ups (after M10, still local-first)

| Order | What |
|-------|------|
| 1 | Real downstream HTTP (mock server in Compose or **`httptest`**) |
| 2 | Outbox **`failed`** + publisher retry / DLQ |
| 3 | Observability: **`event_id`** across all four layers in logs |

**Cloud (paused):** K8s publisher/worker, MSK Terraform — resume when AWS/ROSA available; see **[17-kafka-outbox.md](17-kafka-outbox.md)** post-M9 steps 2–7.

---

## Key files (planned)

| Existing | New / changed |
|----------|----------------|
| **`cmd/worker/main.go`** | DB store, completion before commit |
| **`internal/config/worker.go`** | **`DATABASE_URL`** |
| **`internal/store/`** | Claim / mark completion methods; Phase 1b interface split |
| **`Makefile`** | **`worker-run`** **`DATABASE_URL`** |
| **`README.md`** | M10 E2E + four-layer DB check |
| | **`migrations/00003_consumer_completions.sql`** |
