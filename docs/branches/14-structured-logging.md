# Branch 14: Structured logging and observability (`14-structured-logging`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 6**: application-level observability - JSON logs to **stdout**, request-scoped **HTTP middleware**, Stripe **event_id** / outcome fields, **Recover** aligned with response-wrapping. **No** Datadog/ELK/CloudWatch install in this repo.

**Git branch:** `14-structured-logging`

---

## Phase 0 - Learn (before code)

Use this section as a checklist. Tick items as you go; record final choices under **Decisions (lock before Phase 1)**.

### 0.1 Log pipeline (transfers to cloud)

```text
cmd/api writes JSON lines to stdout/stderr
  -> container runtime on the node
  -> cluster logging agent (varies by platform)
  -> central store (Datadog / ELK / CloudWatch / OpenShift logging / Loki, etc.)
  -> engineer searches by event_id, request_id, level, msg
```

**Takeaway:** M6 owns **log shape and fields**. The employer picks the **collector and UI** (e.g. Immediate: Datadog moving toward ELK; some frontends on CloudWatch).

### 0.2 Read in this repo

| File | Why |
|------|-----|
| **`cmd/api/main.go`** | Today: **`Recover(app.routes())`** - plan **`Recover(RequestLog(app.routes()))`**. |
| **`cmd/api/recover.go`** | Panic after **`WriteHeader`** / body bytes - **`http.Error`** may be wrong; M6 must detect **response started**. |
| **`cmd/api/handlers.go`** | Current **`log.Printf`** on webhook path - will move to structured fields. |
| **`cmd/api/webhook_test.go`** | **`apiHandler()`** must match **production middleware stack** after M6. |

### 0.3 HTTP middleware (request-scoped logging)

- **Middleware** wraps the handler chain: assign **request ID**, wrap **`ResponseWriter`** (status, bytes, started flag), log **request start** / **request end** (duration, status).
- **Context** carries **request_id** (and logger) into **`handleStripeWebhook`**.
- **Probes** **`/livez`** / **`/readyz`**: decide whether access logs run (recommend **skip** or **debug** only to avoid Sandbox noise).

### 0.4 Questions to answer before Phase 1

All rows in **Decisions (lock before Phase 1)** are **locked** (see **PLAN** Decisions **2026-05-27**). Phase 1 code can start.

---

## Decisions (lock before Phase 1)

Record outcomes here; copy dated bullets into **PLAN.md** [Decisions](../../PLAN.md#decisions) when locked.

| Topic | Options | Choice (fill in) |
|-------|---------|------------------|
| **Request ID** | Generate per request (e.g. UUID) / honor **`X-Request-Id`** if present, else generate | **Locked:** honor **`X-Request-Id`** when non-empty and valid (trim, length cap); else **UUID v4**. Attach to **`context`**; log **`request_id`** on every line. **`event_id`** separate after **`ConstructEvent`**. See **PLAN** Decisions **2026-05-27**. |
| **Logger** | **`log/slog`** JSON to stdout (recommended) / other | **Locked:** **`slog.NewJSONHandler(os.Stdout, …)`**; migrate all **`cmd/api`** logging from **`log`** to **`slog`**. **`App`** field **`*slog.Logger`** (base); middleware stores per-request logger on **`context`** (includes **`request_id`**). See **PLAN** Decisions **2026-05-27** (logging). |
| **Access logs for probes** | Skip **`/livez`** **`/readyz`** / log at debug | **Locked:** skip access-style **`request_*`** logs for probe paths (kubelet hits every few seconds). **`POST /webhooks/stripe`** and other routes get full middleware logs. See **PLAN** Decisions **2026-05-27** (access logs). |
| **Stripe trace key** | Always log **`event_id`** after **`ConstructEvent`**; also log **`request_id`** on every line | **Locked:** **`request_id`** everywhere; **`event_id`** + **`event_type`** after verify on **`POST /webhooks/stripe`** (and on **`request_completed`** for that path). Failed verify: **`request_id`** only. **`event_id`** = trace across retries/M7/M8. See **PLAN** Decisions **2026-05-27** (Stripe tracing). |
| **Handler stack order** | **`Recover(RequestLog(routes))`** - Recover outermost | **Locked:** **`handler := Recover(RequestLog(app.routes()))`** in **`main`**; **`apiHandler()`** in tests must match. See **PLAN** Decisions **2026-05-27** (handler stack). |

**Never log:** **`STRIPE_WEBHOOK_SECRET`**, raw webhook body, card numbers, full **`Stripe-Signature`** header.

---

## Log field contract (draft)

Stable **`msg`** values (adjust in Phase 1 if needed):

| `msg` | When | Example fields |
|-------|------|----------------|
| `request_started` | Middleware entry | `request_id`, `method`, `path`, `remote_addr` |
| `request_completed` | Middleware exit | `request_id`, `method`, `path`, `status`, `duration_ms` |
| `stripe_event_verify_failed` | **`ConstructEvent`** or read error | `request_id`, `error` (sanitized) |
| `stripe_event_accepted` | Verified event | `request_id`, `event_id`, `event_type` |
| `panic` | **Recover** | `request_id`, `error`, `response_started` |

Common envelope: `time`, `level`, `msg`, plus handler-specific keys. **No** secrets.

---

## Implementation phases (code)

| Phase | Scope | Status |
|-------|--------|--------|
| **0** | Learn (this doc) | In progress |
| **1** | **`slog`** JSON foundation; **`App.logger`**; context helper; replace **`log`** in **`main`**, **`handlers`**, **`recover`** | Not started |
| **2** | Response wrapper + **RequestLog** middleware; wire **`main`** | Not started |
| **3** | **`handleStripeWebhook`** structured events; quiet probes | Not started |
| **4** | **Recover** + tests (**`apiHandler`** stack) | Not started |
| **5** | Verify local + **`oc logs`**; example searches below | Not started |

---

## How to verify (after Phase 5)

```bash
go test ./...
go run ./cmd/api   # .env with STRIPE_WEBHOOK_SECRET
```

Stripe test event (Sandbox or CLI), then:

```bash
oc logs deployment/go-stripe-webhook-k8s --tail=50
# optional: pipe through jq if installed
```

**Done when:** one delivery traceable by **`event_id`** (and **`request_id`**) without secrets in lines.

### Example searches (documentation only)

**Today (cluster):**

```bash
oc logs deployment/go-stripe-webhook-k8s --tail=200 | grep 'evt_'
oc logs deployment/go-stripe-webhook-k8s --tail=200 | grep 'stripe_event_accepted'
```

**Later (central system - illustrative, not installed here):**

```text
event_id:evt_1Tbgh0Iq4hctS9aMZ2I9FOnP
msg:stripe_event_accepted
level:error
request_id:<uuid>
```

Adapt syntax to ELK/KQL, Datadog, or CloudWatch Logs Insights when your platform is chosen.

---

## Files expected to change (Phases 1-5)

- **`cmd/api/app.go`** (**`*slog.Logger`** on **`App`**), **`cmd/api/main.go`**, **`cmd/api/handlers.go`**, **`cmd/api/recover.go`**
- New: **`cmd/api/middleware.go`** and/or **`cmd/api/logging.go`** (context key + **`loggerFromContext`**; names TBD)
- **`cmd/api/webhook_test.go`** (and new middleware tests)
- **`PLAN.md`** Milestone 6 status; **Decisions** bullets
- This file (status table + locked decisions)

## Follow-ups

- **Milestone 7:** Idempotency - logs should show duplicate **`event_id`** attempts.
- **Stretch:** CI deploy + secret automation (**PLAN** Stretch section).
