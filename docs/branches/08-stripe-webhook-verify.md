# Branch 08: Stripe webhook signature verification (`8-stripe-webhook-verify`)

**Goal:** Finish **Milestone 2** verification: **`POST /webhooks/stripe`** validates **`Stripe-Signature`** on the **raw body** using **`stripe.ConstructEvent`** and **`STRIPE_WEBHOOK_SECRET`**, and returns **400** on invalid or missing signatures (same pattern as Stripe's webhook examples).

## What we added

- **`cmd/api/handlers.go`**: **`github.com/stripe/stripe-go/v85`** **`stripe.ConstructEvent(body, sigHeader, app.cfg.StripeWebhookSecret)`** after **`io.ReadAll`**; **400** on verification / payload errors; **204** on success; logging uses **`event_id`**, **`type`**, **`body_bytes`**, **`remote_addr`**.
- **`cmd/api/webhook_test.go`**: **`newSignedStripeWebhookRequest`** using **`stripe.GenerateTestSignedPayload`**; **`validStripeWebhookJSON`** includes **`api_version`** matching **`stripe.APIVersion`**; tests for **valid signed** POST (**204**), **missing signature** (**400**), **invalid signature** (**400**), invalid JSON (**400**), wrong method.
- **`go.mod` / `go.sum`**: **`stripe-go/v85`** dependency.

## Files changed (high level)

- **`cmd/api/handlers.go`**, **`cmd/api/webhook_test.go`**, **`go.mod`**, **`go.sum`**, **`PLAN.md`**, **`docs/branches/`**

## How to verify

```bash
go test ./...
go run ./cmd/api   # with .env: STRIPE_WEBHOOK_SECRET
```

Send a real event with **Stripe CLI** or Dashboard so **`Stripe-Signature`** matches your secret; ad-hoc **`curl`** without a valid signature should receive **400**.

## Follow-ups

- **Milestone 3:** **`Dockerfile`** and container runbook.
- Optional: reject overly old timestamps / tune **`ConstructEvent`** tolerance if needed beyond library defaults.
