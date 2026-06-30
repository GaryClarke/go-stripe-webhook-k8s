# Branch 17: Kafka + outbox (`17-kafka-outbox`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 9** — decouple webhook **ingestion** from **async processing** using a **Kafka-compatible** broker locally (**Redpanda**) and a **transactional outbox** so accepted Stripe events are never lost if publish fails.

**Git branch:** `17-kafka-outbox`

**Status:** **In progress** — branch doc and decisions locked; implementation not started.

**Prerequisite:** **Milestone 8** complete on **`main`** (**`processed_events`**, Goose, **`/readyz`**, ROSA + RDS proof optional for M9 done gate).

---

## Decisions (locked for M9)

| Decision | Choice |
|----------|--------|
| **Local broker** | **[Redpanda](https://redpanda.com/)** in **`docker-compose.yaml`** — Kafka protocol, no Zookeeper/KRaft setup; describe as *Kafka-compatible broker for local development* |
| **Publish pattern** | **Transactional outbox** — not direct publish-after-commit (avoids Postgres committed + Kafka failed + Stripe got **204**) |
| **Three state layers** | **`processed_events`** = ingestion / idempotency; **`outbox_events`** = durable publish queue; **consumer** = downstream work (M9: log only; completion state deferred) |
| **Ledger status (Option A)** | After accept TX: **`accepted`** on **`processed_events`** — **not** “all downstream work finished” |
| **Rename `processed`** | Migration: allow **`accepted`**; migrate existing **`processed`** rows → **`accepted`**; reserve **`processed`** for a **future downstream** table or consumer completion (not M9) |
| **HTTP — first success** | **204** = *accepted durably for async processing* (signature valid, dedupe won, outbox row committed) |
| **HTTP — duplicate `accepted`** | **204** + **`stripe_event_duplicate_skipped`** |
| **HTTP — existing `processing`** | **204** + **`stripe_event_already_processing`** (legacy in-flight; should be rare after outbox TX) |
| **HTTP — DB / infra error** | **500** (Stripe retries) |
| **Idempotency key (edge)** | Stripe **`event.id`** — unchanged from M8; Kafka does **not** replace Postgres dedupe |
| **Message shape** | **`engine.Job`** JSON on the topic (**`stripe_event_id`**, **`event_type`**, **`payload`**) |
| **Job mapping** | **`cmd/api`** maps **`stripe.Event`** → **`engine.Job`** after **`ConstructEvent`** — keep **`internal/store`** on string primitives; avoid Stripe imports in **`internal/engine`** if adding new helpers |
| **Topic (local)** | e.g. **`stripe-events`** — single topic for M9 |
| **Consumer delivery** | **At-least-once** — worker must be idempotent (key on **`stripe_event_id`**) |
| **Publisher** | Separate process **`cmd/publisher`** (or named **`cmd/outbox-publisher`**) — poll unpublished outbox rows, publish, mark published |
| **Worker** | **`cmd/worker`** — consumer group, read topic, handle **`Job`** (M9: structured log) |
| **M9 done gate** | **Local only:** **`stripe listen`** → API → outbox → publisher → Kafka → worker; README steps documented |
| **ROSA / managed Kafka** | **Out of scope** for M9 done gate |

**Never log:** **`DATABASE_URL`**, raw webhook body, **`STRIPE_WEBHOOK_SECRET`**, Kafka credentials.

**Why outbox:** M8 safe TX (claim → work → commit) breaks when external IO (Kafka) sits inside or immediately after commit without durability. Outbox makes “must publish” as durable as “must dedupe”.

**Why `accepted` not `processed` on ledger:** **`processed`** implied end-to-end completion in M8 when work was trivial in-process. M9 async pipeline needs clear language: **accepted** at edge, **published** on outbox, **consumed** in worker.

---

## Architecture (target)

```text
Stripe POST /webhooks/stripe
  -> verify signature (ConstructEvent)
  -> BEGIN
       INSERT processed_events (status=processing) ON CONFLICT DO NOTHING
       if inserted:
         INSERT outbox_events (pending, payload = Job JSON)
         UPDATE processed_events SET status=accepted, processed_at=now()
       COMMIT
  -> if inserted + committed: log stripe_event_accepted -> 204
  -> if conflict: SELECT status
       accepted    -> log stripe_event_duplicate_skipped -> 204
       processing  -> log stripe_event_already_processing -> 204
       failed      -> (later) retry policy
  -> DB error -> 500

cmd/publisher (loop)
  -> SELECT outbox WHERE status=pending (FOR UPDATE SKIP LOCKED or poll)
  -> publish Job JSON to Kafka topic
  -> UPDATE outbox SET status=published, published_at=now()

cmd/worker (consumer group)
  -> read message from topic
  -> unmarshal Job
  -> idempotent handle (M9: log stripe_job_consumed or similar)
  -> commit offset
```

```text
Layer summary:
  processed_events  = "Did we accept this event_id from Stripe?"
  outbox_events     = "What must still be published to Kafka?"
  consumer logs     = "Did the worker see the job?" (M9)
  (later)           = downstream "processed" / business outcome table
```

---

## Schema

### Ledger migration (M9b)

Goose e.g. **`migrations/00002_ledger_accepted_and_outbox.sql`**:

```sql
-- Extend ledger CHECK; migrate processed -> accepted (M8 rows meant ingested, not worker-done)
ALTER TABLE processed_events DROP CONSTRAINT processed_events_status_check;
ALTER TABLE processed_events ADD CONSTRAINT processed_events_status_check
  CHECK (status IN ('processing', 'accepted', 'processed', 'failed'));
UPDATE processed_events SET status = 'accepted' WHERE status = 'processed';

-- Outbox: one row per accepted event (1:1 with event_id for M9)
CREATE TABLE outbox_events (
    id             BIGSERIAL PRIMARY KEY,
    event_id       TEXT NOT NULL UNIQUE REFERENCES processed_events(event_id),
    event_type     TEXT NOT NULL,
    payload        JSONB NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at   TIMESTAMPTZ,
    CONSTRAINT outbox_events_status_check
      CHECK (status IN ('pending', 'published', 'failed'))
);
```

**Note:** Keep **`processed`** in CHECK temporarily for zero-downtime migration; new code writes **`accepted`** only. Follow-up migration can drop **`processed`** from CHECK once all paths updated.

### Redpanda (Compose)

Add service e.g. **`redpanda`** with Kafka API on host **`9092`** (document exact ports in README). **`depends_on`** / health where useful; API and worker use **`KAFKA_BROKERS=localhost:9092`** locally.

---

## Log field contract (additions)

| `msg` | When | Example fields |
|-------|------|----------------|
| **`stripe_event_accepted`** | Edge TX committed (**`accepted`** + outbox **pending**) | `request_id`, `event_id`, `event_type` |
| **`stripe_event_duplicate_skipped`** | Ledger row exists, **`status = accepted`** | `request_id`, `event_id`, `event_type` |
| **`outbox_publish_succeeded`** | Publisher marked row **published** | `event_id`, `topic`, `partition`, `offset` (if available) |
| **`outbox_publish_failed`** | Publish or mark failed | `event_id`, `error` (sanitized) |
| **`stripe_job_consumed`** | Worker handled message (M9: log) | `event_id`, `event_type`, `offset` |

Existing M8 messages stay; update duplicate branch from **`processed`** → **`accepted`**.

---

## Testing strategy

| Layer | Approach |
|-------|----------|
| **Unit** | Fake store + fake publisher interface; handler and publisher logic without broker |
| **Integration (Postgres)** | Accept TX writes ledger + outbox; duplicate → one outbox row |
| **Integration (Kafka)** | Testcontainers Redpanda/Kafka or Compose up in CI optional; minimum: documented manual path with **`make db-up`** + Compose Redpanda |
| **E2E (local)** | **`stripe listen --forward-to localhost:8080/webhooks/stripe`**; run API + publisher + worker; grep logs for accept → publish → consume |

---

## Implementation order

Work in sequence. **M9a** before **M9b**.

### Phase 0 — Learn (before code)

**Learn:** Kafka topics, partitions, offsets, consumer groups, at-least-once; outbox vs direct publish failure mode; why **204 ≠ downstream done**.

**Read:**

| File | Why |
|------|-----|
| **`cmd/api/handlers.go`** | Current webhook + **`ProcessEvent`** |
| **`internal/store/postgres.go`** | M8 TX claim pattern |
| **`internal/engine/job.go`** | Queue message shape |
| **`PLAN.md` Milestone 9** | Done when |

**Done gate:** Can draw three layers and explain why **`accepted`** replaces **`processed`** on the ledger.

---

### M9a — Kafka basics (local)

#### Phase 1 — Redpanda in Compose

**Do:**

1. Add **`redpanda`** and **`console`** to **`docker-compose.yaml`**.
2. Makefile targets: **`kafka-up`**, **`kafka-down`**, **`kafka-check`**, **`kafka-smoke`** ( **`db-up`** starts the full stack).
3. Document ports in **`docker-compose.yaml`** header and **README.md**.

**Done gate:** **`make kafka-smoke`** succeeds; Console at **http://localhost:8888** shows **`stripe-events`**.

---

#### Phase 2 — Consumer spike (`cmd/worker`)

**Do:**

1. Add **`cmd/worker/main.go`** — connect to broker, join consumer group, read **`stripe-events`**, log payload.
2. Config: **`KAFKA_BROKERS`**, **`KAFKA_TOPIC`**, **`KAFKA_GROUP_ID`** (env, validated at startup).
3. Use a minimal Go Kafka client (e.g. **`segmentio/kafka-go`** — add dep when implementing).

**Done gate:** Manual publish (CLI or spike producer) → worker logs message.

---

#### Phase 3 — Producer spike (optional standalone)

**Do:**

1. Tiny publish script or **`cmd/publish-spike`** — send sample **`engine.Job`** JSON to topic.
2. Confirms serialization matches **`engine.Job`** tags.

**Done gate:** Worker consumes spike message with correct **`stripe_event_id`**.

---

### M9b — Outbox + wired pipeline

#### Phase 4 — Schema + store outbox

**Do:**

1. Goose migration **`00002_...`** (ledger **`accepted`**, **`outbox_events`**).
2. Extend **`internal/store`** (or **`internal/outbox`**) — insert outbox in same TX as accept; publisher queries pending rows.
3. Refactor **`ProcessEvent`** (or new **`AcceptEvent`**) — claim → insert outbox → **`accepted`**.

**Done gate:** Integration test — webhook accept → one **`accepted`** row + one **`pending`** outbox row.

---

#### Phase 5 — Wire webhook handler

**Do:**

1. After **`ConstructEvent`**, build **`engine.Job`** from **`stripe.Event`** in **`cmd/api`**.
2. Replace trivial **`fn`** with outbox insert (inside store TX).
3. Update duplicate handling for **`accepted`**.

**Done gate:** **`go test ./cmd/api/...`** + integration duplicate test green.

---

#### Phase 6 — Outbox publisher (`cmd/publisher`)

**Do:**

1. Poll or **`FOR UPDATE SKIP LOCKED`** on **`pending`** rows.
2. Publish to Kafka; on success **`published`** + log **`outbox_publish_succeeded`**.
3. On failure: log, leave **pending** (simple retry on next poll — no DLQ in M9).

**Done gate:** Accept event via HTTP → publisher → row **published**; message on topic.

---

#### Phase 7 — Worker wired to `Job`

**Do:**

1. Unmarshal **`engine.Job`**; log **`stripe_job_consumed`** with **`event_id`**.
2. Idempotent no-op if same **`event_id`** seen again (log **`stripe_job_duplicate_skipped`** optional).

**Done gate:** Full local path documented in README.

---

#### Phase 8 — README + branch verify

**Do:**

1. README section: start Compose (Postgres + Redpanda), migrate, run **`cmd/api`**, **`cmd/publisher`**, **`cmd/worker`**, **`stripe listen`**.
2. Update **`PLAN.md`** M9 status when done gate met.

**Done gate:** **[PLAN.md](../../PLAN.md) Milestone 9 done when** — `Stripe webhook → Kafka → consumer` locally, documented.

---

## Out of scope (M9)

- ROSA / ECR deploy of publisher or worker
- Managed Kafka (MSK, Redpanda Cloud)
- Dead-letter queue, exponential backoff, exactly-once semantics
- Downstream **`processed`** completion table (consumer business outcome)
- Stale **`processing`** / **`failed`** recovery workflows
- Outbox multi-row per event, saga, or cross-service transactions
- Removing **`internal/engine.StripeEvent`** / legacy parse path (optional cleanup later)

---

## Key files (planned)

| Existing | New / changed |
|----------|----------------|
| **`cmd/api/handlers.go`** | Accept + outbox in TX; **`accepted`** duplicate branch |
| **`internal/store/`** | **`accepted`** status; outbox insert in TX; publisher queries |
| **`migrations/00001_...`** | Unchanged history |
| **`docker-compose.yaml`** | **Redpanda** service |
| **`internal/engine/job.go`** | Possibly **`Job`** builder from id/type/payload without **`StripeEvent`** |
| | **`migrations/00002_ledger_accepted_and_outbox.sql`** |
| | **`cmd/worker/`** |
| | **`cmd/publisher/`** |
| | **`internal/kafka/`** or **`internal/queue/`** (producer/consumer wrappers) |
| **`README.md`** | Local M9 runbook |

---

## Verify checklist

- [ ] Phase 0: Three layers + **204 semantics** understood
- [x] M9a Phase 1: Redpanda in Compose healthy
- [ ] M9a Phase 2: Worker consumes test message
- [ ] M9a Phase 3: Job JSON spike end-to-end
- [ ] M9b Phase 4: Migration + outbox in accept TX
- [ ] M9b Phase 5: Webhook → **`accepted`** + outbox **pending**
- [ ] M9b Phase 6: Publisher → Kafka → outbox **published**
- [ ] M9b Phase 7: Worker logs **`stripe_job_consumed`**
- [ ] M9b Phase 8: README local E2E + **`stripe listen`**

---

## Follow-ups

- **Consumer completion table** — record downstream **`processed`** / **`failed`** separately from ingestion ledger
- **ROSA stretch** — run publisher + worker on cluster; or single Deployment with sidecars (document tradeoffs)
- **M8 doc alignment** — **`16-idempotency-postgres.md`** historical **`processed`** wording vs M9 **`accepted`** (comment in migration README)
- **Observability stretch** — **`event_id`** on publisher/worker logs tied to **`request_id`** where possible
