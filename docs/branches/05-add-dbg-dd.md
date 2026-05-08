# Branch 05: Debug dump helper (`internal/dbg.DD`)

**Goal:** Add a **`dd()`-style** helper for **handlers and tests** behind **`Recover`**: dump values with **`spew`** to **stderr**, then **`os.Exit(1)`**, without using **panic** (so **`Recover`** does not hide the output).

## What we added

- **`internal/dbg/dd_debug.go`** - **`//go:build debug`**: **`DD`** uses **`github.com/davecgh/go-spew`** and **`os.Exit(1)`**.
- **`internal/dbg/dd_release.go`** - **`//go:build !debug`**: **`DD`** is a **no-op** so default **`go build`** / **`go test`** and release images have **no** dump or exit path.
- **`go.mod`**: direct **`go-spew`** dependency (compiled only when **`debug`** tag is set).
- **`PLAN.md`**: Milestone 1 **Build** row, **Local-only scratch files** note (**`zzz_stripe_webhook_k8s_dd_scratch_only.go`** for personal global gitignore), **Decisions** entry.

## Files changed (high level)

- **`internal/dbg/`**, **`go.mod`**, **`go.sum`**, **`PLAN.md`**

## How to verify

```bash
go test ./...
go test -tags debug ./...
```

With **`debug`**, call **`dbg.DD(...)`** from code under test or **`go run -tags debug ./cmd/api`** and confirm **stderr** output and exit **1**.

Release / CI: **omit** **`-tags debug`** (plain **`go build ./...`** is enough).

## Follow-ups

- **Milestone 2:** **`STRIPE_WEBHOOK_SECRET`**, **`PORT`**, **`DOWNSTREAM_URL`**, config wiring.
- Do **not** leave permanent **`dbg.DD`** calls in handlers; use only while debugging (reviews / grep).
