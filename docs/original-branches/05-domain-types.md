# Branch 05: Domain types

**Goal:** Define Stripe event and internal job structs, add fixtures. Phase 1 begins. No HTTP or queue yet; just the data shapes.

## What we're adding

- **StripeEvent** - Top-level webhook payload (id, type, data, created, livemode)
- **StripeEventData** - Wraps data.object as json.RawMessage (flexible for multiple event types)
- **Job** - Internal work item for the queue (StripeEventID, EventType, Payload)
- **Fixture** - testdata/stripe-invoice-payment-succeeded.json

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| data.object type | json.RawMessage | Multiple event types have different shapes (Invoice, Charge, etc.). Defer parsing or forward as-is. |
| Job name | Job | Idiomatic for queue/worker systems. Source-agnostic (not tied to webhooks). |
| Relay vs process | Relay | This app forwards; downstream parses and does business logic. Keeps engine focused. |

## Steps (in order)

1. Add StripeEvent and StripeEventData to internal/engine/stripe.go.
2. Add Job to internal/engine/job.go.
3. Add fixture testdata/stripe-invoice-payment-succeeded.json (from Stripe docs or `stripe trigger invoice.payment_succeeded`).
4. Run `go build ./...` to verify.

## Files changed/added

- `internal/engine/stripe.go` - StripeEvent, StripeEventData
- `internal/engine/job.go` - Job
- `testdata/stripe-invoice-payment-succeeded.json` - Sample invoice.payment_succeeded event

## Next branch

**06-config** - Env-based configuration and validation (godotenv, QUEUE_BACKEND, etc.). See [06-config.md](06-config.md).
