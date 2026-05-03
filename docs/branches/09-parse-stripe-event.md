# Branch 09: Parse Stripe event (webhook body)

**Goal:** Decode raw webhook JSON bytes into `StripeEvent`—the shared core used later from Lambda (`event.Body`) and from local HTTP (`r.Body`). No `Job`, queue, or `cmd/*` wiring in this slice.

## What we're adding

- **ParseStripeEvent** - `json.Unmarshal` into `StripeEvent`, wrap errors with `fmt.Errorf` and `%w`
- **Tests** - Fixture `testdata/stripe-invoice-payment-succeeded.json`; invalid JSON returns error

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| File name | `stripe_parse.go` | Sits next to `stripe.go` types; clear responsibility. |
| Validation | None in v1 | Unmarshal + tests only; field checks can follow in a later branch. |
| Fixture path in tests | `../../testdata/...` from `internal/engine` | Reuses repo root `testdata`; `go test` cwd is the package directory. |

## Steps (in order)

1. Add `internal/engine/stripe_parse.go` with `ParseStripeEvent`.
2. Add `internal/engine/stripe_parse_test.go` with fixture and invalid JSON cases.
3. Run `go test ./internal/engine/...` and `go build ./...`.

## Files changed/added

- `internal/engine/stripe_parse.go` - `ParseStripeEvent`
- `internal/engine/stripe_parse_test.go` - Tests

## Next branch

**10** — `StripeEvent` → `Job` mapping (proposed scope). Full Phase 1 proposal (**10–16**) is in [PROJECT_KNOWLEDGE.md §5 — Proposed branches](../PROJECT_KNOWLEDGE.md#proposed-branches-phase-1-remainder).
