# Branch 01: Foundation

**Goal:** Project layout, Go module, tooling, and conventions so we can build incrementally with a single source of truth.

## What was added

- **Go module** - `go mod init integration-engine`
- **Directory structure**
  - `cmd/` - Future entry points (ingest, ingest-local, worker, worker-local)
  - `internal/` - Shared code; started with `internal/engine` (placeholder package)
  - `testdata/` - Fixtures (e.g. Stripe payloads)
  - `docs/` - Project knowledge and branch logs
- **Tooling**
  - `Makefile` - `build`, `test`, `lint`, `tidy`
  - `.golangci.yml` - Linters: errcheck, gofmt, govet, ineffassign, staticcheck, unused
  - `.gitignore` - Go, IDE, env, Terraform, OS
- **Docs**
  - `docs/PROJECT_KNOWLEDGE.md` - Plan, local vs Lambda, Lambda invocation
  - `docs/README.md` - Doc index
  - Root `README.md` - Repo overview and quick start

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| Module path | `integration-engine` | Simple, no GitHub dependency; can change to `github.com/owner/repo` later if needed. |
| Layout | Standard Go: `cmd/`, `internal/`, `testdata/` | Idiomatic; `internal` for shared code, `cmd` for executables. |
| Linting | golangci-lint, optional in `make lint` | If not installed, `make lint` still runs `go vet`; CI can require golangci-lint. |
| Placeholder package | `internal/engine` with `doc.go` | Ensures `go build ./...` passes and documents intended use of `internal`. |

## How to run

```bash
go mod tidy
make build
make test
make lint
```

## Files changed/added

- `go.mod`
- `.gitignore`
- `Makefile`
- `.golangci.yml`
- `README.md`
- `internal/engine/doc.go`
- `testdata/.gitkeep`
- `docs/PROJECT_KNOWLEDGE.md`
- `docs/README.md`
- `docs/original-branches/01-foundation.md`

## Next branch

**02-domain-types** - Stripe event and internal job structs; fixtures in `testdata/`.
