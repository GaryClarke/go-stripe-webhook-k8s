# Branch 04: Stripe webhook stub (`POST /webhooks/stripe`)

**Goal:** Complete the **Milestone 1** application surface: **`POST /webhooks/stripe`** as a **stub** (no signature verification yet), reuse **`engine.ParseStripeEvent`**, keep **`204`** on success, and align routing with production tests.

## What we added

- **`handleStripeWebhook`** in **`cmd/api/handlers.go`**: **`http.MaxBytesReader`** (**1 MiB** cap), **`engine.ParseStripeEvent`**, **400** / **413** on bad input / oversize, **204** on success.
- **Logging:** **`event_id`**, **`type`**, **`body_bytes`**, **`stripe_signature_present`** (boolean only), **`remote_addr`** - no **`Stripe-Signature`** header value logged.
- **`cmd/api/app.go`**: **`App`**, **`routes()`**; **`cmd/api/handlers.go`** probe + webhook handlers; **`cmd/api/recover.go`** **`Recover`** - **`main.go`** is lifecycle + **`Recover(app.routes())`**.
- **`cmd/api/webhook_test.go`**: **`apiHandler()`** = same stack as **`main`** (**`Recover(app.routes())`** with test **`App`**), **`ServeHTTP`** on Stripe and **`/livez`** cases.
- **`PLAN.md`**: **`cmd/api` layout** note and evolution toward **`App`** + **`routes()`**.

## Files changed (high level)

- **`cmd/api/handlers.go`**, **`cmd/api/app.go`**, **`cmd/api/main.go`**, **`cmd/api/webhook_test.go`** (new), **`PLAN.md`**

## How to verify

```bash
go test ./...
go run ./cmd/api
curl -sS -i -X POST http://localhost:8080/webhooks/stripe \
  -H 'Content-Type: application/json' \
  --data @testdata/stripe-invoice-payment-succeeded.json
```

Expect **204** and a structured log line with **`event_id`** / **`type`**.

## Follow-ups

- **Milestone 2:** **`STRIPE_WEBHOOK_SECRET`**, signature verification, **`PORT`**, **`DOWNSTREAM_URL`**.
- **`internal/dbg.DD`** - see [05-add-dbg-dd.md](05-add-dbg-dd.md) (build tag **`debug`**; no permanent calls in prod paths).
