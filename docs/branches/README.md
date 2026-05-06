# Branch docs (Kubernetes / HTTP service — index)

Work proceeds in **small, numbered git branches** that each map to **one unit of learning** and one **`NN-descriptive-name.md`** file in this folder.

- **Git branch names** use the same number and slug, e.g. `1-add-livez-and-readyz`.
- **Learning roadmap and milestones:** see **[PLAN.md](../../PLAN.md)** (source of truth for what “Milestone 1”, etc. means).
- **Lambda-era branch logs** (original project) live under **[docs/original-branches/](../original-branches/README.md)** — same idea, different phase of the repo.

## Cursor workflow

All chat shorthands and merge behaviour: **[cursor-rules.md](../../cursor-rules.md)** only. This index stays about **what** each branch documents, not command definitions.

## Branch write-ups (this track)

Add a row when you open or complete a branch.

| Branch (git) | Doc | Topic |
|--------------|-----|--------|
| `1-add-livez-and-readyz` | [01-add-livez-and-readyz.md](01-add-livez-and-readyz.md) | `GET /livez`, `GET /readyz`, stdlib JSON probes |
| `2-graceful-shutdown` | [02-graceful-shutdown.md](02-graceful-shutdown.md) | `http.Server`, SIGINT/SIGTERM, `Shutdown` with timeout |
| `3-recover-panic` | [03-recover-panic.md](03-recover-panic.md) | `Recover` middleware, `httptest`, no debug `/panic` route |

**Doc template:** mirror the style in [original-branches](../original-branches/README.md): goal, scope, key files, how to verify, follow-ups.

## Cross-links

- [docs/README.md](../README.md) — docs index  
- [docs/PROJECT_KNOWLEDGE.md](../PROJECT_KNOWLEDGE.md) — historical Lambda architecture and original build order §5  
