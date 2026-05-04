# Branch docs (Kubernetes / HTTP service — index)

Work proceeds in **small, numbered git branches** that each map to **one unit of learning** and one **`NN-descriptive-name.md`** file in this folder.

- **Git branch names** use the same number and slug, e.g. `1-add-livez-and-readyz`.
- **Learning roadmap and milestones:** see **[PLAN.md](../../PLAN.md)** (source of truth for what “Milestone 1”, etc. means).
- **Lambda-era branch logs** (original project) live under **[docs/original-branches/](../original-branches/README.md)** — same idea, different phase of the repo.

## How we work with Cursor

When a branch is **finished** (reviewed, ready to merge), say:

```text
ms <branch-name>
```

Example: `ms 1-add-livez-and-readyz`. Treat that as the signal to merge to the parent line, tidy docs, or prep the next branch—as you prefer.

## Branch write-ups (this track)

Add a row when you open or complete a branch.

| Branch (git) | Doc | Topic |
|--------------|-----|--------|
| `1-add-livez-and-readyz` | — | *Add doc when you merge* — `GET /livez`, `GET /readyz`, stdlib HTTP |

**Doc template:** mirror the style in [original-branches](../original-branches/README.md): goal, scope, key files, how to verify, follow-ups.

## Cross-links

- [docs/README.md](../README.md) — docs index  
- [docs/PROJECT_KNOWLEDGE.md](../PROJECT_KNOWLEDGE.md) — historical Lambda architecture and original build order §5  
