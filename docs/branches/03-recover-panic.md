# Branch 03: Panic recovery (middleware)

**Goal:** Ensure an unexpected panic in any HTTP handler becomes a **500** response and a **logged stack** instead of **crashing the process**, and lock that behaviour in with a focused **unit test of `Recover`**, not production debug routes.

## What we added

- **`Recover(next http.Handler) http.Handler`** in **`cmd/api`** - **`defer` / `recover()`**, **`log.Printf`** with **`debug.Stack()`**, **`http.Error`** for a **generic** client body (no panic string leaked).
- **`handler := Recover(mux)`** before **`http.Server{ Handler: handler }`** so **`/livez`** and **`/readyz`** (and future routes) are covered.
- **`cmd/api/recover_test.go`** - **`httptest`**:
  - **`TestRecover_PanicReturns500`** - synthetic **`GET /boom`** handler that panics; expect **500** and **Internal Server Error** text.
  - **`TestRecover_NoPanicPassesThrough`** - normal **204** response still works through the wrapper.
- **Removed** temporary **`GET /panic`** from the live mux (manual testing replaced by unit tests).

## Files changed

- **`cmd/api/main.go`** - **`Recover`**, wire-up, **`runtime/debug`** import.
- **`cmd/api/recover_test.go`** - new.

## How to verify

```bash
go test ./cmd/api/...
go run ./cmd/api
curl -sS http://localhost:8080/livez
```

No **`/panic`** route should exist on the running server.

## Follow-ups

- **`POST /webhooks/stripe`** (stub or real) to complete Milestone 1 **Build** in **`PLAN.md`**.
- **`PORT`** from env (Milestone 2).
- **`Chain`** or more middleware when you add request IDs / access logs (see Milestone 6).
