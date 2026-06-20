# Branch 16: Idempotency with Postgres (`16-idempotency-postgres`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 8** — durable webhook idempotency using **Postgres** and Stripe **`event.id`**; event processing **ledger** with **`status`**; prove dedupe across **multiple Pods** on ROSA.

**Git branch:** `16-idempotency-postgres`

**Status:** **In progress** — Phases **1–5** shipped (Compose, Goose, **`internal/store`**, webhook wired with fake-store unit tests). **`downstream/`** removed (admin SPA lives in its own repo). Next: Phase **6** (duplicate integration tests), then **`/readyz`** + K8s **`DATABASE_URL`**.

**Prerequisite:** **Milestone 7** mostly shipped on **`main`** (ROSA deploy, CI, lab scripts). **M7 Phase 5 done gate** can close in parallel — does **not** block local M8 work (Phases 0–7).

---

## Repo cleanup (this branch)

The **`downstream/`** folder was an early convenience copy for a separate admin service. That service now has its **own repository** — remove **`downstream/`** from this repo so M8 Postgres/Compose is scoped to the **webhook API** only.

---

## Decisions (locked for M8)

| Decision | Choice |
|----------|--------|
| **Idempotency key** | Stripe **`event.id`** (e.g. **`evt_...`**) |
| **Store** | **Postgres** — durable, shared across Pods, audit trail (not Redis, not in-memory, not SQLite) |
| **Table** | **Event processing ledger** — not a minimal dedupe-only table |
| **Claim pattern** | **Insert-first** — **`INSERT … ON CONFLICT DO NOTHING`**; DB **`UNIQUE`** is the lock (no SELECT-then-INSERT) |
| **M8 processing** | **Single transaction:** claim (**`processing`**) → trivial in-process work → **`processed`** → **COMMIT** |
| **HTTP — first success** | **204 No Content** (after transaction commits **`processed`**) |
| **HTTP — duplicate `processed`** | **204** + log **`stripe_event_duplicate_skipped`** |
| **HTTP — existing `processing`** | **204** + log **`stripe_event_already_processing`** (in-flight; do not force Stripe retries) |
| **HTTP — DB / infra error** | **500** (Stripe should retry) |
| **Statuses (initial)** | **`processing`**, **`processed`**, **`failed`** ( **`failed`** wired for schema; recovery workflows deferred) |
| **Migrations** | **[Goose](https://github.com/pressly/goose)** — SQL-first, **`migrations/`**, same migrations for local / test / RDS |
| **Local dev DB** | **Docker Compose** Postgres at repo root — **not** the removed **`downstream/`** stack |
| **Test DBs** | Separate DB names, e.g. **`stripe_webhook_dev`**, **`stripe_webhook_test`** — same **`DATABASE_URL`** pattern |
| **Testing** | **Unit:** store fakes/mocks (no DB). **Integration:** real Postgres (constraints + **`ON CONFLICT`** behaviour) |
| **Config** | **`DATABASE_URL`** required at runtime (fail fast in **`config.Load`**) |
| **Readiness** | **`/readyz`** pings Postgres; **`/livez`** stays process-only |
| **ROSA proof** | **`replicas: 2+`** in **`k8s/overlays/rosa`** after RDS wired |
| **Driver (suggested)** | **`database/sql`** + **`github.com/jackc/pgx/v5/stdlib`** |
| **Outbox** | **Not M8** — discuss direct Kafka vs outbox in **Milestone 9** |

**Never log:** **`DATABASE_URL`** (credentials), raw webhook body, **`STRIPE_WEBHOOK_SECRET`**.

**Why single transaction for M8:** Handler has **no external IO** yet (no Kafka, no downstream HTTP). Claim + work + **`processed`** in one TX means a pod crash **rolls back** — no committed **`processing`** left behind. **`processing` → 204** covers concurrent duplicates; stale **`processing`** recovery matters when M9 adds commit-then-publish (outbox).

---

## Architecture (target)

```text
Stripe POST /webhooks/stripe (may retry / race across Pods)
  -> verify signature (ConstructEvent)
  -> BEGIN
       INSERT processed_events (event_id, event_type, status=processing, received_at)
         ON CONFLICT DO NOTHING
       if inserted:
         -- M8: trivial work only (log); M9+: outbox / publish moves outside TX
         UPDATE status=processed, processed_at=now()
       COMMIT
  -> if inserted + committed: log stripe_event_accepted -> 204
  -> if conflict: SELECT status
       processed   -> log stripe_event_duplicate_skipped -> 204
       processing  -> log stripe_event_already_processing -> 204
       failed      -> (later) retry policy
  -> DB error -> 500

Multiple Pods + shared Postgres:
  -> one INSERT wins; others conflict and branch on status
```

---

## Schema (M8 — ledger)

Goose migration e.g. **`migrations/00001_create_processed_events.sql`**:

```sql
CREATE TABLE processed_events (
    event_id       TEXT PRIMARY KEY,
    event_type     TEXT NOT NULL,
    status         TEXT NOT NULL,
    received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at   TIMESTAMPTZ,
    failed_at      TIMESTAMPTZ,
    error_message  TEXT
);

-- Optional: CHECK (status IN ('processing', 'processed', 'failed'))
```

Apply locally:

```bash
goose -dir migrations postgres "$DATABASE_URL" up
```

Same command against test DB and future RDS.

---

## Log field contract (additions)

| `msg` | When | Example fields |
|-------|------|----------------|
| `stripe_event_duplicate_skipped` | Row exists, **`status = processed`** | `request_id`, `event_id`, `event_type` |
| `stripe_event_already_processing` | Row exists, **`status = processing`** | `request_id`, `event_id`, `event_type` |
| `stripe_event_process_failed` | TX or update failed (before **500**) | `request_id`, `event_id`, `error` (sanitized) |
| `readyz_db_check_failed` | **`/readyz`** cannot ping DB | `request_id`, `error` (sanitized) |

Existing **`stripe_event_accepted`** when this Pod claims and commits **`processed`**.

---

## Testing strategy

| Layer | Approach |
|-------|----------|
| **Unit** | Fake **`Store`** — claims event, reports duplicate, returns errors; handler tests without Postgres |
| **Integration** | Real Postgres: compose up → migrate **`stripe_webhook_test`** → run tests → truncate or isolated schema per package |
| **Why real DB for integration** | **`ON CONFLICT`**, uniqueness, and transaction behaviour are Postgres guarantees — mocks cannot prove them |

Integration tests skip or short-circuit when **`DATABASE_URL`** / test DB unavailable (document in Makefile).

---

## Implementation order

Work **in this sequence**. Each phase has a **done gate** before the next.

### Phase 0 — Learn (before code)

**Learn:** Stripe retries; **`event.id`** vs **`request_id`**; insert-first vs check-then-insert races; why single TX fits M8; when outbox matters (M9).

**Read in this repo:**

| File | Why |
|------|-----|
| **`cmd/api/handlers.go`** | Webhook flow — verify → **`ProcessEvent`** / **`Status`** → **204** or **500** |
| **`cmd/api/logmsg.go`** | New **`msg`** constants |
| **`internal/config/config.go`** | **`DATABASE_URL`** |
| **`PLAN.md` Milestone 8** | Done when criteria |

**Done gate:** Can explain claim pattern, status branching, and why **204** for both **`processed`** and **`processing`**.

---

### Phase 1 — Local Postgres (Docker Compose)

**Do:**

1. Add **`docker-compose.yml`** (Postgres **16**, healthcheck, host port e.g. **`5433:5432`**).
2. DB/user e.g. **`stripe_webhook_dev`**; document **`DATABASE_URL`**.
3. **`Makefile`** targets: **`db-up`**, **`db-down`**, **`db-migrate`** (wraps Goose).

**Done gate:** **`make db-up`** → healthcheck passes.

---

### Phase 2 — Goose migrations

**Do:**

1. Add **`migrations/00001_create_processed_events.sql`** (ledger schema above).
2. Add Goose dependency; document **`goose … up`** in this file and README/Makefile.
3. Separate **`DATABASE_URL`** for test DB in integration test env.

**Done gate:** **`make db-migrate`** → **`\d processed_events`** shows all columns.

---

### Phase 3 — Store interface

**Do:**

1. Package **`internal/store`** with interface + Postgres implementation.
2. Suggested API:

   ```go
   // ProcessEvent claims event_id in a transaction, runs fn when this caller wins the claim,
   // then marks processed. Returns (claimed bool, err error).
   // claimed=false when another row exists (caller inspects status for logging).
   ProcessEvent(ctx context.Context, eventID, eventType string, fn func(ctx context.Context) error) (claimed bool, err error)

   // Status returns current status when event_id exists.
   Status(ctx context.Context, eventID string) (status string, found bool, err error)

   Ping(ctx context.Context) error
   ```

3. Wire store on **`App`** (**`NewApp(cfg, logger, store)`**).

**Done gate:** Unit tests with fake store; interface documented.

---

### Phase 4 — Atomic claim + transaction

**Do:**

1. **`INSERT … ON CONFLICT DO NOTHING`** with **`status = processing`**.
2. On insert: run **`fn`** inside same TX; **`UPDATE`** **`processed`**, **`processed_at`**.
3. On conflict: **`Status`** for handler logging branch.

**Done gate:** Integration test — same **`event_id`** twice → one row, **`status = processed`**.

---

### Phase 5 — Wire webhook handler

**Do:**

1. After **`ConstructEvent`**, call **`ProcessEvent`** with **`fn`** = log-only for M8.
2. Branch logs + **204** per decisions table; DB errors → **500**.

**Done gate:** **`go test ./cmd/api/...`** with fake store (**shipped**). Duplicate delivery against real Postgres → Phase **6**.

---

### Phase 6 — Integration tests (duplicate + concurrency)

**Do:**

1. Real Postgres + migrations on **`stripe_webhook_test`**.
2. Same signed webhook twice → one row, both **204**, correct log **`msg`** on second call.
3. Optional: concurrent POSTs same **`event.id`** → still one row.

**Done gate:** Integration tests green with **`make db-up`**.

---

### Phase 7 — `/readyz` DB check

**Do:**

1. **`handleReadyz`** calls **`store.Ping`**; failure → **503** + **`readyz_db_check_failed`**.
2. **`handleLivez`** unchanged.

**Done gate:** Postgres stopped → **`/readyz`** fails.

---

### Phase 8 — Kubernetes: `DATABASE_URL`

**Do:**

1. ~~**`config.Load`:** require **`DATABASE_URL`**.~~ **Done in Phase 5** (local + **`go run`**).
2. **`k8s/base/deployment.yaml`:** env from Secret **`database-url`**.
3. **`deploy-rosa.yaml`:** sync secret from GitHub (match **`stripe-webhook-secret`** pattern).
4. Run Goose migrations against RDS/DB before or during deploy (document approach).

**Done gate:** Pod **Ready** on cluster; webhook persists events.

---

### Phase 9 — RDS for ROSA (Terraform)

**Do:**

1. **`infra/terraform/rds/`** (separate state).
2. Networking: ROSA workers → RDS.
3. **`DATABASE_URL`** via GitHub Secret — never in git.

**Done gate:** **`processed_events`** rows on RDS from live webhook.

---

### Phase 10 — Multi-replica proof

**Do:**

1. **`replicas: 2`** in **`k8s/overlays/rosa`** (keep **`1`** in **`k8s/base`**).
2. Duplicate / concurrent deliveries → one row; logs show one accept + duplicates skipped or in-flight.

**Done gate:** **[PLAN.md](../../PLAN.md) Milestone 8 done when** satisfied.

---

## Out of scope (M8)

- **Outbox pattern** / Kafka publish (**Milestone 9**)
- **Stale `processing` reclaim** / **`failed`** retry workflows (schema ready; logic later)
- **Redis** cache
- **External Secrets Operator** (stretch)
- **Downstream admin SPA** (separate repo)

---

## Key files (existing vs new)

| Existing | New / changed |
|----------|----------------|
| **`cmd/api/handlers.go`** | **Shipped:** ledger in webhook; **pending:** **`/readyz`** ping (Phase 7) |
| **`cmd/api/app.go`**, **`main.go`** | **Shipped:** inject **`store.Store`** |
| **`internal/config/config.go`** | **Shipped:** **`DATABASE_URL`** required |
| **`cmd/api/logmsg.go`** | **Shipped:** idempotency + **`store_init_failed`** **`msg`** constants |
| **`k8s/base/deployment.yaml`** | **Pending:** **`DATABASE_URL`** env (Phase 8) |
| **`k8s/overlays/rosa`** | **Pending:** **`replicas: 2`**, RDS secret (Phases 9–10) |
| **`.github/workflows/deploy-rosa.yaml`** | **Pending:** **`database-url`** secret (Phase 8) |
| | **`docker-compose.yaml`** — **shipped** |
| | **`migrations/`** + Goose — **shipped** |
| | **`internal/store/`** — **shipped** |
| | **`infra/terraform/rds/`** — **pending** (Phase 9) |

---

## Verify checklist

- [x] Phase 0: Decisions understood
- [x] Phase 1: Compose Postgres healthy
- [x] Phase 2: Goose migration applied
- [x] Phase 3: Store interface + unit fakes
- [x] Phase 4: TX claim + **`processed`** integration test
- [x] Phase 5: Webhook wired — **204** for processed + processing duplicates (unit tests; duplicate integration → Phase 6)
- [ ] Phase 6: Integration duplicate delivery
- [ ] Phase 7: **`/readyz`** DB check
- [ ] Phase 8: K8s **`DATABASE_URL`**
- [ ] Phase 9: RDS on ROSA
- [ ] Phase 10: **`replicas: 2+`** cross-Pod dedupe

---

## Follow-ups

- **Milestone 9:** Kafka — direct publish vs **outbox** (Barclays-shaped durability).
- **M7 close-out:** **`make lab-off`** → CI skip → **`make lab-on`** → deploy — mark M7 complete in **PLAN.md**.
