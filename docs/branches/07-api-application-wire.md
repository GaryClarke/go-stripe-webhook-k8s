# Branch 07: API application wiring (`7-api-application-wire`)

**Goal:** Introduce an **`App`** type that holds **`internal/config.Config`**, register routes on a dedicated **`(*App).routes()`** **`ServeMux`**, split handler implementations out of the old mux module, and wrap the root handler with **`Recover`** in **`main`** and in tests (same stack as production).

## What we added

- **`cmd/api/app.go`**: **`App`**, **`NewApp`**, **`(*App).routes()`** with **`GET /livez`**, **`GET /readyz`**, **`POST /webhooks/stripe`**.
- **`cmd/api/handlers.go`**: probe and Stripe webhook handlers (evolved from **`mux.go`**; removes **`newMux`**-style setup from that file).
- **`cmd/api/recover.go`**: package-level **`Recover(next http.Handler)`** middleware (panic log + **500**); **`main`** uses **`Recover(app.routes())`**.
- **`cmd/api/main.go`**: wires **`Recover`** around **`app.routes()`** instead of a bare mux.
- **`cmd/api/webhook_test.go`**: **`apiHandler()`** builds **`NewApp(testConfig)`** and returns **`Recover(app.routes())`** so **`httptest`** exercises the same stack as **`main`**.
- **`PLAN.md`**, **`docs/branches/README.md`**, **`03-recover-panic.md`**, **`04-webhook-stripe-stub.md`**: references updated from **`mux.go`** / **`newMux`** to **`App`** and **`routes()`**.

## Files changed (high level)

- **`cmd/api/app.go`** (new), **`cmd/api/handlers.go`** (renamed / split from **`mux.go`**), **`cmd/api/recover.go`** (new), **`cmd/api/main.go`**, **`cmd/api/webhook_test.go`**, **`PLAN.md`**, **`docs/branches/*`**

## How to verify

```bash
go test ./...
go run ./cmd/api
```

Confirm **`GET /livez`**, **`GET /readyz`**, and **`POST /webhooks/stripe`** behave as before; panics in handlers still return **500** without killing the process (**`Recover`**).

## Follow-ups

- **`8-stripe-webhook-verify`**: **`stripe.ConstructEvent`** and **`STRIPE_WEBHOOK_SECRET`** at the webhook boundary (see [08-stripe-webhook-verify.md](08-stripe-webhook-verify.md)).
- Pass additional dependencies on **`App`** (downstream clients, publishers) as milestones add them.
