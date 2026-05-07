# Branch 01: Liveness and readiness (`livez`, `readyz`)

**Goal:** Introduce the Kubernetes-oriented HTTP entrypoint **`cmd/api`**, **`http.ServeMux`** (Go 1.22+ method and path patterns), and probe endpoints **`GET /livez`** and **`GET /readyz`** so we can match **`livenessProbe`** / **`readinessProbe`** paths later.

## What we added

- **`cmd/api/main.go`** as the primary local binary (alongside transitional **`cmd/ingest`** Lambda entry).
- **`GET /livez`** and **`GET /readyz`** returning **JSON** via **`encoding/json`** and a shared **`healthResponse`** shape (`{"status":"ok"}`).
- **`ListenAndServe`** on **`:8080`** for a minimal blocking server (later superseded by **`http.Server`** and graceful shutdown in **branch 02**).
- Repo hygiene from the same era: **`docs/original-branches/`** for Lambda branch logs, fresh **`docs/branches/`** index, **`PLAN.md`** / **`README.md`** pointers to **`cursor-rules.md`** for chat shorthands (**`ms`**, **`mmp`**, etc.).

## Files changed (high level)

- **`cmd/api/main.go`** - probes and first HTTP server.
- **`README.md`**, **`PLAN.md`**, **`docs/branches/README.md`**, **`docs/README.md`**, **`docs/PROJECT_KNOWLEDGE.md`**, **`cursor-rules.md`** - K8s learning path and doc layout.

## How to verify

```bash
go run ./cmd/api
curl -sS http://localhost:8080/livez
curl -sS http://localhost:8080/readyz
```

Expect **HTTP 200** and JSON **`{"status":"ok"}`** for both (readiness intentionally matches liveness until dependencies exist; see **`PLAN.md`** Milestone 1 note).

## Follow-ups

- **02-graceful-shutdown** - **`http.Server`**, signals, **`Shutdown`**. See [02-graceful-shutdown.md](02-graceful-shutdown.md).
- **03-recover-panic** - **`Recover`**, **`httptest`**. See [03-recover-panic.md](03-recover-panic.md).
- **04-webhook-stripe-stub** - **`POST /webhooks/stripe`**. See [04-webhook-stripe-stub.md](04-webhook-stripe-stub.md).
