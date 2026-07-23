# Branch 19: Downstream HTTP (`19-downstream-http`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 11** — replace the **`handleJob`** log stub with a **real HTTP downstream call**, classify failures, and preserve **M10 completion + offset discipline** so external side effects are safe under Kafka at-least-once delivery.

**Git branch:** `19-downstream-http`

**Status:** **Not started** — decisions locked; implementation pending.

**Prerequisite:** **Milestone 10** complete on **`main`** (**`consumer_completions`**, idempotent worker, offset after DB).

**Cloud:** **Out of scope** — local Compose only (Postgres + Redpanda). No ROSA, AWS, MSK, or K8s deploy in this branch.

---

## Decisions (locked for M11)

| Decision | Choice |
|----------|--------|
| **Downstream transport** | **HTTP POST** from **`handleJob`** — **not** a second Kafka publish in this milestone |
| **Why not Kafka downstream** | M10 closed one pipeline (Stripe → outbox → Kafka → worker → completion). A second topic adds another outbox/completion boundary before proving one external side effect. Kafka fan-out deferred to **M14+** if desired. |
| **Dev / test downstream** | **`httptest.Server`** in unit tests; optional small mock in Compose for manual E2E (Phase 5 optional) |
| **Real third-party API** | **Out of scope** — no production credentials |
| **Config** | **`DOWNSTREAM_URL`** required in **`LoadWorker()`** when worker runs for real (fail fast). Tests inject URL from **`httptest`** — no env needed in **`go test`**. |
| **Request shape** | **`POST`** JSON body = **`engine.Job`** (or subset: **`stripe_event_id`**, **`event_type`**, **`payload`**) — keep Stripe-free in store; marshal in worker |
| **Success** | HTTP **2xx** → **`MarkConsumerProcessed`** → **`handleRecord` returns `true`** → commit offset (unchanged from M10) |
| **Retryable failure** | Timeout, connection error, **HTTP 429**, **HTTP 5xx** → **`MarkConsumerFailed`** (sanitized **`error`**) → **`handleRecord` returns `false`** → **do not commit** → Kafka redelivers → **`ClaimConsumerCompletion`** **`retry_from_failed`** (existing M10 behaviour; **`attempt_count`** bumps) |
| **Permanent failure** | Malformed job (worker-side), unsupported routing, **HTTP 400/404/422** (client fault) → **`MarkConsumerFailed`** → **`handleRecord` returns `true`** → **commit offset** so the **partition does not stall** on a poison message |
| **Duplicate / already processed** | **`ClaimConsumerCompletion`** → **`already_processed`** → **do not call HTTP** → commit offset (unchanged from M10) |
| **Offset vs DB order** | **Unchanged from M10:** never commit offset before durable completion write (processed or failed) |
| **M10 “all failures no commit” rule** | **Refined in M11:** M10 used a single path (stub always succeeded). M11 splits **retryable** (no commit) vs **permanent** (commit after **`failed`** row). |
| **Schema changes** | **None** — reuse **`consumer_completions`** (**`attempt_count`**, **`error`**, **`status`**) |
| **DLQ topic** | **Out of scope** — deferred to **M13** |
| **Max attempts / backoff** | **Out of scope** — deferred to **M12** ( **`attempt_count`** already incremented on reclaim) |
| **Circuit breaker** | **Out of scope** |
| **Never log** | **`DATABASE_URL`**, **`DOWNSTREAM_URL`** credentials, raw secrets, full **`payload`** (log **`event_id`**, **`event_type`**, HTTP status, error class) |

**Learning goal:** Safely perform an **external side effect** under at-least-once delivery — duplicate skip must **not** double-call downstream; retryable outages must **redeliver**; permanent faults must **not block the partition forever**.

---

## Architecture (target)

```text
Layer summary (unchanged from M10):

  processed_events      = accepted from Stripe
  outbox_events         = published to Kafka
  consumer_completions  = downstream result (processed | failed)
```

```text
cmd/worker (per message — M11 additions in bold)

  -> unmarshal engine.Job
  -> ClaimConsumerCompletion
       already processed -> stripe_job_duplicate_skipped -> commit (no HTTP)
  -> **handleJob: POST DOWNSTREAM_URL**
       2xx -> success
       retryable -> return retryable error
       permanent -> return permanent error
  -> on success: MarkConsumerProcessed -> commit
  -> on retryable: MarkConsumerFailed -> no commit
  -> on permanent: MarkConsumerFailed -> commit
  -> CommitRecords (when handleRecord true)
```

**Outcome matrix (locked):**

| Outcome | `consumer_completions` | `handleRecord` | Commit offset? | HTTP on redelivery |
|---------|------------------------|----------------|----------------|---------------------|
| Duplicate skip | already **`processed`** | `true` | Yes | No |
| HTTP 2xx | **`processed`** | `true` | Yes | N/A |
| Retryable error | **`failed`** → reclaim **`processing`** | `false` | No | Yes (until success or permanent policy in M12) |
| Permanent error | **`failed`** (terminal for this message) | `true` | Yes | No (offset moved past poison) |
| Claim / mark DB error | **`failed`** or none | `false` | No | Depends on row state |

---

## Error classification (locked defaults)

| Class | Examples | HTTP | Offset |
|-------|----------|------|--------|
| **Retryable** | timeout, connection refused, DNS (transient), **429**, **500–599** | — | No commit |
| **Permanent** | missing **`stripe_event_id`**, unsupported **`event_type`** (if routed), **400**, **404**, **422** | — | Commit after **`failed`** |
| **Success** | **200–299** | — | Commit after **`processed`** |

**Note:** Map by **HTTP status** when a response exists; map **network/timeout** errors to retryable when no response.

Implement via a small **`JobError`** (or similar) with **`Retryable() bool`** returned from **`handleJob`** — avoid string matching in **`handleRecord`**.

---

## Phases

### Phase 0 — Lock model (this doc)

**Done gate:** Outcome matrix and retryable vs permanent offset rules understood.

---

### Phase 1 — Config + HTTP client surface

**Do:**

1. **`internal/config/worker.go`** — add required **`DOWNSTREAM_URL`** (trimmed, validated **`http`/`https`**) for **`LoadWorker()`**
2. **`worker_test.go`** — missing/blank **`DOWNSTREAM_URL`** cases
3. Small **`DownstreamClient`** interface (e.g. **`DeliverJob(ctx, job) error`**) + **`httpDownstream`** implementation using **`net/http`** with context timeout
4. Wire client into worker **`main`** (construct from config)

**Done gate:** Worker fails fast without **`DOWNSTREAM_URL`**; client unit-testable with injected **`httptest`** URL.

---

### Phase 2 — `handleJob` HTTP + error types

**Do:**

1. Replace log-only **`handleJob`** with **`POST`** to **`DOWNSTREAM_URL`**
2. Classify errors → retryable vs permanent (see table above)
3. Structured logs: **`stripe_job_handled`** on success; new log e.g. **`stripe_job_downstream_failed`** with **`event_id`**, **`retryable`**, **`http_status`** (no raw payload)
4. **`Makefile`** **`worker-run`** — pass **`DOWNSTREAM_URL`** (e.g. mock URL or documented dev default)

**Done gate:** Manual curl/httptest: 2xx succeeds; 503 returns retryable; 400 returns permanent.

---

### Phase 3 — `handleRecord` offset rules

**Do:**

1. **`handleRecord`** interprets **`handleJob`** outcome:
   - success → **`MarkConsumerProcessed`** → `true`
   - retryable → **`MarkConsumerFailed`** → `false`
   - permanent → **`MarkConsumerFailed`** → `true`
2. Ensure duplicate path never calls **`handleJob`**

**Done gate:** Same completion/offset matrix as doc; no regression on M10 duplicate / claim paths.

---

### Phase 4 — Tests

**Do:**

1. **`handle_job_test.go`** — **`httptest.Server`**: 200, 503, 400; timeout
2. **`handle_record_test.go`** — extend **`fakeCompletionStore`** / fake client: permanent vs retryable → **`handleRecord`** true/false
3. Optional: extend **`completion_integration_test.go`** with HTTP (inject client or local server)

**High-value cases (required):**

| Case | Assert |
|------|--------|
| **2xx** | processed row; **`handleRecord` true**; downstream called once |
| **503** | failed row; **`handleRecord` false**; downstream called |
| **400 permanent** | failed row; **`handleRecord` true** |
| **Duplicate skip** | **`handleJob` / HTTP not invoked** |

**Done gate:** **`go test ./cmd/worker/...`** green; integration tests unchanged or extended.

---

### Phase 5 — Docs + optional E2E

**Do:**

1. **README** — fifth beat: worker POSTs to downstream; env **`DOWNSTREAM_URL`**; note permanent vs retryable behaviour briefly
2. **PLAN.md** — M11 status complete when done gate met
3. Optional: Compose **`downstream-mock`** service for local **`make worker-run`** (single endpoint returns 200)

**Done gate:** Documented local path; optional mock server instructions.

---

## Log messages (M11)

| Message | When |
|---------|------|
| **`stripe_job_handled`** | Downstream HTTP success |
| **`stripe_job_duplicate_skipped`** | Unchanged — no HTTP |
| **`stripe_job_downstream_failed`** (new) | Downstream error — include **`retryable`**, **`http_status`** if any |
| **`consumer_completion_failed`** | Unchanged — mark/claim DB errors |
| **`stripe_job_consumed`** | Unchanged — after successful handle + mark |

---

## Verify checklist

- [ ] Phase 0: Outcome matrix + permanent-failure-commits-offset locked
- [ ] Phase 1: **`DOWNSTREAM_URL`** + **`DownstreamClient`**
- [ ] Phase 2: **`handleJob`** HTTP + classification
- [ ] Phase 3: **`handleRecord`** offset rules
- [ ] Phase 4: Unit tests (2xx, 503, 400, duplicate no HTTP)
- [ ] Phase 5: README + PLAN M11

---

## Out of scope (M11)

- Second Kafka topic / event fan-out
- DLQ topic (**M13**)
- Max attempts / backoff / **`next_retry_at`** (**M12**)
- Outbox **`failed`** publisher retry
- Real third-party SaaS integration
- ROSA / K8s / MSK
- **`consumer_attempts`** history table (latest **`error`** on row is enough for M11)

---

## Follow-ups (after M11)

| Order | Milestone | What |
|-------|-----------|------|
| 1 | **M12** | Retry policy — max attempts, backoff, poison after N tries |
| 2 | **M13** | DLQ topic + replay tooling |
| 3 | — | Outbox **`failed`** + publisher retry |
| 4 | **M14+** | Optional second Kafka boundary / microservice split |
| 5 | — | Phase 1b store interface segregation (from M10) |

**Cloud (paused):** See **[17-kafka-outbox.md](17-kafka-outbox.md)** post-M9 steps.

---

## Key files (planned)

| Existing | New / changed |
|----------|----------------|
| **`cmd/worker/handle_job.go`** | Real HTTP delivery |
| **`cmd/worker/main.go`** | Client wiring; **`handleRecord`** outcome handling |
| **`internal/config/worker.go`** | **`DOWNSTREAM_URL`** |
| **`Makefile`** | **`worker-run`** **`DOWNSTREAM_URL`** |
| **`cmd/worker/handle_job_test.go`** | **`httptest`** cases |
| **`cmd/worker/handle_record_test.go`** | Permanent vs retryable offset behaviour |
| **`README.md`** | M11 E2E notes |
