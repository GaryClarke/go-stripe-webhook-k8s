# Branch 17: Kafka + outbox (`17-kafka-outbox`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 9** — decouple webhook **ingestion** from **async processing** using a **Kafka-compatible** broker locally (**Redpanda**) and a **transactional outbox** so accepted Stripe events are never lost if publish fails.

**Git branch:** `17-kafka-outbox`

**Status:** **In progress** — M9a complete; **M9b Phase 5 complete** (webhook → `AcceptEvent`); Phase 6 (publisher) next.

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
| **Go Kafka client** | **[`twmb/franz-go`](https://github.com/twmb/franz-go)** (`kgo`) — not `segmentio/kafka-go`; one client library for worker + publisher; TLS/SASL/IAM via opts later without changing consume/publish loops |
| **Consumer commit** | **Manual commit after successful handle** — `DisableAutoCommit` + `CommitRecords`; aligns with at-least-once and makes retry-on-failure explicit (M9a spike onward) |
| **Rebalance safety** | **`BlockRebalanceOnPoll`** + **`AllowRebalance`** after batch commit — finish in-flight records before partition handoff |
| **Publisher** | Separate process **`cmd/publisher`** — poll **`pending`** outbox rows, **`ProduceSync`** to Kafka, conditional mark **`published`** (see Phase 6) |
| **Publisher instances (M9)** | **One** local process — no multi-publisher claim; **`FOR UPDATE SKIP LOCKED`** / **`publishing`** state deferred |
| **Publisher delivery (M9)** | **At-least-once** — Kafka ack before mark **`published`**; mark failure → row stays **`pending`** → may duplicate on topic; worker idempotent (Phase 7) |
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
  -> AcceptEvent (one TX)
       INSERT processed_events (status=accepted) ON CONFLICT DO NOTHING
       if inserted:
         INSERT outbox_events (pending, payload = Job JSON)
       COMMIT
  -> if inserted + committed: log stripe_event_accepted -> 204
  -> if conflict: SELECT status
       accepted    -> log stripe_event_duplicate_skipped -> 204
       processing  -> log stripe_event_already_processing -> 204
       failed      -> (later) retry policy
  -> DB error -> 500

cmd/publisher (single instance, poll loop — no DB TX during Kafka)
  -> NextPendingOutbox (SELECT pending ORDER BY id LIMIT 1)
  -> ProduceSync Job JSON to topic (record key = event_id)
  -> MarkOutboxPublished (UPDATE … WHERE event_id AND status=pending)
  -> on Kafka fail: log outbox_publish_failed; row stays pending
  -> on Kafka ok, mark fail: log outbox_mark_published_failed; row stays pending (may duplicate on topic)
  -> on both ok: log outbox_publish_succeeded

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
| **`outbox_publish_succeeded`** | Kafka ack **and** row marked **`published`** | `event_id`, `topic`, `partition`, `offset` (if available) |
| **`outbox_publish_failed`** | **`ProduceSync`** failed (broker/network) | `event_id`, `topic`, `error` (sanitized) |
| **`outbox_mark_published_failed`** | Kafka ack succeeded but **`MarkOutboxPublished`** failed | `event_id`, `topic`, `error` (sanitized) — Kafka may already have the message |
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

1. **`go get github.com/twmb/franz-go/pkg/kgo`**
2. **`internal/config/worker.go`** — **`LoadWorker()`**: **`KAFKA_BROKERS`** (comma-separated), **`KAFKA_TOPIC`**, **`KAFKA_GROUP_ID`**; separate from API **`config.Load()`** (worker has no Stripe/DB).
3. **`cmd/worker/`** — JSON logger + log message constants (mirror **`cmd/api`**); **`main.go`** with **`kgo.NewClient`**:
   - **`SeedBrokers`**, **`ConsumerGroup`**, **`ConsumeTopics`**
   - **`ClientID("stripe-webhook-worker")`** — visible in broker/Console metrics
   - **`DisableAutoCommit`**, **`BlockRebalanceOnPoll`**
   - Loop: **`PollFetches(ctx)`** → unmarshal/log → **`CommitRecords`** → **`AllowRebalance`**
   - Graceful shutdown: **`signal.NotifyContext`** cancels **`PollFetches`**; **`client.Close()`** on exit
4. Makefile **`worker-run`** with local env defaults (same as **`KAFKA_BROKERS`** / **`KAFKA_TOPIC`** in Makefile).

**Local env (defaults):**

```bash
KAFKA_BROKERS=localhost:19092
KAFKA_TOPIC=stripe-events
KAFKA_GROUP_ID=stripe-webhook-worker
```

**Verify:** **`make worker-run`**, then produce a **`engine.Job`** JSON record (see Phase 3 shape); worker logs **`stripe_job_consumed`**. Consumer group visible in Console **http://localhost:8888**.

**Tests:** **`TestLoadWorker`**, **`TestHandleRecord`** (default **`go test ./...`**); **`TestWorker_ConsumeJob_Integration`** behind **`//go:build integration`** — **`make test-integration`** (requires **`make kafka-up`**). CI runs unit tests only; integration skips without a broker.

**Done gate:** Manual publish (CLI or spike producer) → worker logs message; offset committed (re-start worker does not replay unless you produce again).

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

**Shipped (`7dba66a`, `80d67d9`):**

| Area | Delivered |
|------|-----------|
| **Migration** | **`migrations/00002_ledger_accepted_and_outbox.sql`** — ledger **`accepted`** CHECK; migrate M8 **`processed`** → **`accepted`**; **`outbox_events`** (1:1 **`event_id`**, **`payload` JSONB**, **`pending`/`published`/`failed`**) |
| **Store interface** | **`StatusAccepted`**, outbox constants, **`AcceptEvent`**, **`OutboxStatus`** on **`Store`** |
| **Postgres** | **`AcceptEvent`** — one TX: ledger **`accepted`** + outbox **`pending`**; **`OutboxStatus`**; **`TruncateLedger`** truncates **`outbox_events`** + **`processed_events`** (FK-safe) |
| **Tests** | **`TestAcceptEvent_claimThenDuplicate`** — first accept → **`accepted`** + **`pending`**; duplicate → **`claimed=false`**, one row each |

**Phase 4 implementation decisions:**

| Decision | Choice |
|----------|--------|
| **M8 path** | **`ProcessEvent`** kept unchanged for existing tests; webhook switches to **`AcceptEvent`** in Phase 5 |
| **Ledger in accept TX** | Insert straight to **`accepted`** (no **`processing`** hop — outbox insert is the only in-TX work) |
| **Payload** | Caller marshals **`engine.Job`** JSON; store uses **`$3::jsonb`** (Postgres validates JSON) |
| **Dedupe** | Same as M8: **`ON CONFLICT (event_id) DO NOTHING`** → **`claimed=false`**, not an error |
| **Test reset** | **`TRUNCATE outbox_events, processed_events`** in one statement |

---

#### Phase 5 — Wire webhook handler

**Do:**

1. After **`ConstructEvent`**, build **`engine.Job`** from **`stripe.Event`** in **`cmd/api`**.
2. Replace **`ProcessEvent`** with **`AcceptEvent`** (outbox insert inside store TX).
3. Update duplicate handling for **`accepted`**.

**Done gate:** **`go test ./cmd/api/...`** + integration duplicate test green.

**Shipped:** Handler maps **`stripe.Event`** → **`engine.Job`** → **`AcceptEvent`**; duplicate branch handles **`accepted`**; **`fakeStore`** extended; integration tests assert ledger **`accepted`** + outbox **`pending`**.

---

#### Phase 6 — Outbox publisher (`cmd/publisher`)

**Do:**

1. **`internal/store`** — **`NextPendingOutbox`**, **`MarkOutboxPublished`** (conditional UPDATE); integration tests.
2. **`internal/config/publisher.go`** — **`LoadPublisher()`**: **`DATABASE_URL`**, **`KAFKA_BROKERS`**, **`KAFKA_TOPIC`**; optional poll interval.
3. **`cmd/publisher/`** — poll loop, **`kgo`** producer (**`ProduceSync`**), JSON logs, graceful shutdown; Makefile **`publisher-run`**.
4. Fakeable produce wrapper for unit tests (optional but recommended).

**Done gate:** Accept event via HTTP → publisher → row **`published`**; message on topic (worker consume in Phase 7/8).

**Store contract (M9):**

```go
type OutboxRow struct {
    EventID string
    Payload []byte // engine.Job JSON from outbox_events.payload
}

// NextPendingOutbox returns nil, nil when no pending rows.
NextPendingOutbox(ctx context.Context) (*OutboxRow, error)
MarkOutboxPublished(ctx context.Context, eventID string) (updated bool, error)
```

**SQL (mark):**

```sql
UPDATE outbox_events
SET status = 'published', published_at = now()
WHERE event_id = $1 AND status = 'pending';
```

Inspect **`RowsAffected() == 1`** for **`updated`**.

**Failure semantics (M9):**

| Step | Outcome |
|------|---------|
| Kafka fails | Row stays **`pending`**; retry next poll |
| Kafka ok, mark fails | Row stays **`pending`**; retry may **duplicate** on topic |
| Both ok | Row **`published`** |

**Never** mark **`published`** before Kafka acknowledges. **Never** hold a DB transaction open during **`ProduceSync`**.

**Phase 6 implementation decisions:**

| Decision | Choice |
|----------|--------|
| **Publisher instances** | **One** local process (document in README when Phase 8 lands) |
| **Claim / lock** | **None** — no **`FOR UPDATE SKIP LOCKED`**; no **`publishing`** status in M9 |
| **Read pending** | **`ORDER BY id LIMIT 1`** — no TX during Kafka |
| **Mark published** | Conditional UPDATE **`WHERE status = 'pending'`** |
| **Kafka key** | **`event_id`** (partition stickiness; not exactly-once) |
| **Head-of-line blocking** | First failing row blocks later **`pending`** rows — **accepted for M9**; defer **`attempt_count`**, backoff, **`failed`**, skip-to-next |
| **Multi-publisher** | **Deferred** — post-M9: **`publishing`** + **`claimed_at`** + stale reclaim |
| **DLQ / backoff** | **Out of scope** M9 — simple poll retry only |

**Implementation order:**

1. Lock claim model (this section) ✓
2. Store methods + integration tests
3. **`LoadPublisher()`**
4. Fakeable **`ProduceSync`** wrapper
5. Poll loop + logs + shutdown
6. Local done-gate proof (HTTP → published → topic)

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
- **Multi-publisher outbox claims** (`publishing` status, stale reclaim) — M9 runs **one** publisher
- Downstream **`processed`** completion table (consumer business outcome)
- Stale **`processing`** / **`failed`** recovery workflows
- Outbox multi-row per event, saga, or cross-service transactions
- Removing **`internal/engine.StripeEvent`** / legacy parse path (optional cleanup later)

---

## Key files (planned)

| Existing | New / changed |
|----------|----------------|
| **`cmd/api/handlers.go`** | Accept + outbox in TX; **`accepted`** duplicate branch *(Phase 5)* |
| **`internal/store/`** | **`AcceptEvent`** *(Phase 4–5)*; **`NextPendingOutbox`**, **`MarkOutboxPublished`** *(Phase 6)* |
| **`migrations/00001_...`** | Unchanged history |
| **`docker-compose.yaml`** | **Redpanda** service *(M9a)* |
| **`internal/engine/job.go`** | Possibly **`Job`** builder from id/type/payload without **`StripeEvent`** |
| | **`migrations/00002_ledger_accepted_and_outbox.sql`** *(Phase 4)* |
| | **`cmd/worker/`** *(M9a)* |
| | **`cmd/publisher/`** |
| | **`internal/kafka/`** — shared **`kgo`** client opts (brokers, TLS/SASL from env; M9b+ when publisher lands) |
| **`README.md`** | Local M9 runbook |

---

## Verify checklist

- [ ] Phase 0: Three layers + **204 semantics** understood
- [x] M9a Phase 1: Redpanda in Compose healthy
- [x] M9a Phase 2: Worker consumes test message
- [x] M9a Phase 3: Job JSON spike end-to-end *(skipped — `rpk` + worker integration test)*
- [x] M9b Phase 4: Migration + outbox in accept TX
- [x] M9b Phase 5: Webhook → **`accepted`** + outbox **pending**
- [ ] M9b Phase 6: Publisher → Kafka → outbox **published**
- [ ] M9b Phase 7: Worker logs **`stripe_job_consumed`**
- [ ] M9b Phase 8: README local E2E + **`stripe listen`**

---

## Follow-ups

Work **after** M9 **done gate** (local E2E documented). **Do not start these until M9b Phase 8 passes** — order matters.

### Portable code rule (apply during M9, not a follow-up)

- **`cmd/api`**, **`cmd/publisher`**, **`cmd/worker`** share no "local mode" branches in Go.
- **Local vs prod** = **env vars + deploy manifests only** (brokers, topic, group, TLS/SASL when needed).
- **Redpanda in Compose** = dev infra; clients use **`KAFKA_BROKERS`** (local: **`localhost:19092`**; prod: MSK/broker endpoints).

---

### Post-M9 follow-ups (recommended order)

| Step | Track | What | Why this order |
|------|-------|------|----------------|
| **1** | **App hardening** | Consumer completion table; DLQ / retry policy for outbox **`failed`**; **multi-publisher claims** (`publishing`, `claimed_at`, stale reclaim); **`attempt_count`** / backoff for head-of-line; stale **`processing`** reclaim (if needed) | Finish **behaviour** locally before new infra |
| **2** | **K8s / ROSA** | **`k8s/`** Deployments (or separate overlays) for **`cmd/publisher`** + **`cmd/worker`**; Secrets for **`KAFKA_*`** + publisher **`DATABASE_URL`**; extend **`deploy-rosa.yaml`** or sibling workflow (ECR tags for extra binaries) | Run async pipeline on cluster **before** managed broker Terraform |
| **3** | **Broker choice** | Decide **MSK** vs **Redpanda Cloud** vs in-VPC self-managed; document in **`docs/`** | Informs Terraform and **`KAFKA_BROKERS`** in prod |
| **4** | **Terraform** | e.g. **`infra/terraform/kafka/`** (MSK cluster, SG, bootstrap brokers output) — separate state key like RDS | Broker reachable from ROSA VPC workers |
| **5** | **Kafka auth** | TLS + SASL (or IAM for MSK) via env: e.g. **`KAFKA_TLS_ENABLED`**, **`KAFKA_SASL_*`** — wire in shared **`internal/kafka`** client config | Same consumer/publisher code; stricter broker in prod |
| **6** | **Secrets automation** | GitHub / **External Secrets** for **`KAFKA_*`** and outbox DB (align with PLAN OpenShift deploy stretch) | No manual **`oc create secret`** each lab session |
| **7** | **Observability** | **`event_id`** on publisher/worker logs; consumer **lag** alerts (Console locally; CloudWatch/Prometheus in prod) | Ops narrative after pipeline runs in prod |

---

### Post-M9 detail (by track)

**Kubernetes / ROSA (step 2)**

- Separate Deployments vs sidecars in API pod — document tradeoff (independent scale vs ops simplicity).
- Publisher needs **private RDS** access (same VPC pattern as M8 **`cmd/api`**).
- Worker needs **Kafka** reachability only (no Stripe secret).
- CI: build/push **`cmd/publisher`** and **`cmd/worker`** images (mirror **`api`** / **`migrate`** tag pattern).

**Managed Kafka / Terraform (steps 3–4)**

- **MSK** in same AWS account/VPC as ROSA (parallel to **`infra/terraform/rds/`**).
- Outputs: bootstrap broker list → GitHub Secret or **`KAFKA_BROKERS`** Variable.
- **Not in M9:** no MSK module until local outbox + publisher + worker path is proven.

**TLS / SASL (step 5)**

- Local Redpanda: plaintext on **`localhost:19092`** (Compose only).
- Prod: enable TLS/SASL in client dialer from env — **no rewrite** of consume/publish loops.
- Document required env vars in README when implemented.

**App / data model (step 1)**

- **Consumer completion table** — downstream **`processed`** / **`failed`** separate from ingestion **`accepted`** ledger.
- **Outbox **`failed`** rows** — retry counter, DLQ topic, or manual replay (pick one when needed).
- **Multi-publisher** — durable claim (`publishing`), stale **`claimed_at`** recovery; do not use **`FOR UPDATE SKIP LOCKED`** without a persisted claim state.

**Docs / cleanup (anytime after M9)**

- **`16-idempotency-postgres.md`** — historical **`processed`** wording vs M9 **`accepted`**.
- Optional: remove **`internal/engine.StripeEvent`** if unused.

**Observability (step 7)**

- Tie **`event_id`** across API → outbox → publisher → worker where possible.
- **Consumer lag** as primary Kafka health metric (learned via Redpanda Console locally).
