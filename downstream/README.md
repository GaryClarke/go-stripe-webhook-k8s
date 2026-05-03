# Downstream admin service (bootstrap pack)

This folder is a **portable bootstrap pack** for a separate repository: an **admin-style** downstream that receives normalized events from the [Integration Engine](../README.md) (eventually) and exposes an **internal HTTP API** + **Angular** UI for operators.

Copy this entire `downstream/` tree into your new repo root when you are ready.

---

## 1. Project

**Goal:** Persist **customers** and **invoices** derived from Stripe-style webhook traffic (starting with invoice events), and provide a small **read API** for an Angular app (list/filter invoices, view detail, browse customers).

**Phase focus (now):** Ship the **internal read API** + **PostgreSQL** schema, migrations, and seed data so you can run the stack locally and point **Angular** at real HTTP endpoints with proper loading and error handling. **Ingest** from the integration engine is specified here but implemented after the read path is solid.

**Phase later:** `POST` ingest endpoint (machine-to-machine), Auth0 client-credentials or API-key auth, ECS deployment, wire the Go worker’s HTTP forwarder to this service.

---

## 2. Architecture

```mermaid
flowchart LR
  subgraph engine [Integration Engine - other repo]
    WH[Webhook ingest]
    Q[Queue]
    W[Worker forwarder]
  end

  subgraph downstream [This service - new repo]
    API[Go HTTP API]
    DB[(PostgreSQL)]
    FE[Angular SPA]
  end

  W -->|"POST /ingest (later)"| API
  FE -->|"GET /api/* (now)"| API
  API --> DB
```

| Layer | Responsibility |
|--------|----------------|
| **PostgreSQL** | Source of truth for `customers` and `invoices`. |
| **Go API** | Internal REST API for the SPA; later, ingest + idempotent upserts from forwarded **Job** payloads. |
| **Angular** | Browser UI: tables, filters (dates, amounts, status), detail views, HttpClient + error handling. |
| **Docker Compose** | Local **Postgres** (required); optional **API** service once a `Dockerfile` exists. |

**Local dev (recommended early on):**

- Compose: **Postgres only** (or Postgres + API when containerised).
- **Angular:** Node on the host (`ng serve`) with `environment.ts` pointing at the Go API URL (e.g. `http://localhost:8080`).

---

## 3. Data model

### 3.1 Tables (initial)

**`customers`**

| Column | Type | Notes |
|--------|------|--------|
| `id` | `UUID` PK | Internal id |
| `stripe_customer_id` | `TEXT` UNIQUE NOT NULL | Stripe `cus_...` |
| `email` | `TEXT` | Nullable early on |
| `name` | `TEXT` | Nullable |
| `created_at` | `TIMESTAMPTZ` | |
| `updated_at` | `TIMESTAMPTZ` | |

**`invoices`**

| Column | Type | Notes |
|--------|------|--------|
| `id` | `UUID` PK | Internal id |
| `stripe_invoice_id` | `TEXT` UNIQUE NOT NULL | Stripe `in_...` |
| `customer_id` | `UUID` FK → `customers.id` | |
| `status` | `TEXT` NOT NULL | e.g. `paid`, `open`, `void` (mirror Stripe) |
| `amount_due` | `BIGINT` NOT NULL | Stripe minor units (e.g. pence/cents) |
| `amount_paid` | `BIGINT` NOT NULL | Same |
| `currency` | `CHAR(3)` NOT NULL | ISO 4217 lowercase e.g. `gbp` |
| `stripe_created` | `TIMESTAMPTZ` NOT NULL | Invoice `created` from Stripe (UTC) |
| `due_date` | `TIMESTAMPTZ` | Nullable |
| `paid_at` | `TIMESTAMPTZ` | Nullable; from `status_transitions.paid_at` when present |
| `created_at` | `TIMESTAMPTZ` | Row insert |
| `updated_at` | `TIMESTAMPTZ` | Last update (webhook or API) |

**Indexes (initial):** `customers(stripe_customer_id)` unique; `invoices(stripe_invoice_id)` unique; `invoices(customer_id)`; `invoices(stripe_created DESC)` for list screens; optional `invoices(status)`.

**Optional later:** `webhook_deliveries(stripe_event_id UNIQUE, ...)` for idempotent ingest.

Full DDL and **seed data** live in [`docs/schema.sql`](docs/schema.sql).

### 3.2 Integration engine → downstream (future ingest)

The Go worker forwards a **Job** envelope (see [`fixtures/job-invoice-payment-succeeded.example.json`](fixtures/job-invoice-payment-succeeded.example.json)):

- `stripe_event_id` — idempotency / tracing  
- `event_type` — e.g. `invoice.payment_succeeded`  
- `payload` — typically Stripe **`data.object`** JSON (here, an **Invoice**)

The ingest handler will parse `payload`, upsert **customer** from `invoice.customer`, upsert **invoice** from invoice fields, and return **201** / **409** (duplicate event) as appropriate. That work is **out of scope for the first milestone** in the new repo; only the contract is frozen in this pack.

---

## 4. Technology

| Area | Choice |
|------|--------|
| Database | **PostgreSQL 16** (local via Docker; RDS/Aurora on AWS later) |
| API | **Go 1.22+** (stdlib `net/http` or small router; `pgx` or `database/sql` + driver) |
| Migrations | **goose**, **golang-migrate**, or plain SQL in CI — pick one in the new repo |
| Frontend | **Angular** (current LTS); Node via **nvm** / Volta; `package.json` + lockfile pin versions |
| Container runtime (prod) | **ECS** (task: API image; DB managed) |
| Local orchestration | **Docker Compose** — Postgres; API optional |

---

## 5. Internal API (Phase 1 — build this first)

Base URL example: `http://localhost:8080`

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/health` | Liveness (e.g. `200` + `{ "status": "ok" }`) |
| `GET` | `/api/customers` | List customers (pagination optional v1) |
| `GET` | `/api/customers/{id}` | Customer by internal UUID |
| `GET` | `/api/invoices` | List invoices; query params: `from`, `to` (ISO dates), `status`, `stripe_customer_id` or `customer_id` |
| `GET` | `/api/invoices/{id}` | Invoice by internal UUID |

**Errors:** JSON body e.g. `{ "error": { "code": "NOT_FOUND", "message": "..." } }` with appropriate HTTP status.

**CORS:** Allow `http://localhost:4200` (Angular dev server) on `/api/*`.

**Auth (v1):** None or a single shared dev header; add **Auth0 M2M** / API key before exposing beyond localhost.

---

## 6. Docker & Docker Compose

### 6.1 What runs where

- **`docker compose up`:** Starts **PostgreSQL** immediately.  
- **Go API:** Run with `go run ./cmd/api` on the host (simplest), or add a `Dockerfile` and an `api` service to Compose once the binary exists.  
- **Angular:** `ng serve` on the host until you add a production static build + nginx or embed in the API image.

### 6.2 Files in this pack

- [`docker-compose.yml`](docker-compose.yml) — `db` service + optional commented pattern for `api`.

**Connection string (example):**

```text
postgres://downstream:downstream@localhost:5433/downstream?sslmode=disable
```

(Port **5433** avoids clashing with a local Postgres on 5432; change if you prefer.)

---

## 7. Quick start (after you move this into a new repo)

1. **Start Postgres:** `docker compose up -d`  
2. **Apply schema + fixtures:** `psql "$DATABASE_URL" -f docs/schema.sql` (or run migrations + seed script)  
3. **Implement and run** the Go API (Phase 1 routes above).  
4. **Angular:** create app under `web/`, set `apiBaseUrl`, build list/detail + HttpClient error handling.

---

## 8. Contents of this folder

| Path | Purpose |
|------|---------|
| `README.md` | This document |
| `docs/schema.sql` | DDL + seed rows |
| `fixtures/job-invoice-payment-succeeded.example.json` | Example **Job** body for future ingest |
| `docker-compose.yml` | Local Postgres |

---

## 9. Relation to Integration Engine (this repo)

The engine’s job type is defined in Go as:

```go
type Job struct {
    StripeEventID string          `json:"stripe_event_id"`
    EventType     string          `json:"event_type"`
    Payload       json.RawMessage `json:"payload"`
}
```

Sample Stripe invoice payload shape is under [`../testdata/stripe-invoice-payment-succeeded.json`](../testdata/stripe-invoice-payment-succeeded.json). The ingest implementation should treat `payload` as the invoice object (or resolve event type and branch as needed).

When the new repo exists, add a one-line link back to this engine repo in its README for portfolio context.
