# Branch docs (index)

Branch **numbers match git branch names** (e.g. `7-queue-abstraction`, `8-queue-from-config`) and **`docs/branches/NN-*.md`**. The **canonical roadmap** with Phase context is **[PROJECT_KNOWLEDGE.md §5](../PROJECT_KNOWLEDGE.md#5-build-order-branches)**.

**Completed** branch docs are listed in the table below. **Proposed** branches **10–16** (Phase 1 remainder) and observability timing are spelled out in **§5 → “Proposed branches (Phase 1 remainder)”**—add a new `NN-*.md` here when each ships.

| Branch doc | Topic |
|------------|--------|
| [01-foundation.md](01-foundation.md) | Go module, layout, tooling (Phase 0) |
| [02-api-gateway.md](02-api-gateway.md) | API Gateway + `GET /healthz` (Phase 0) |
| [03-s3-backend-ci.md](03-s3-backend-ci.md) | Terraform remote state (S3 + lock); CI may be separate or folded |
| [05-domain-types.md](05-domain-types.md) | `StripeEvent`, `Job`, fixtures — Phase 1 starts |
| [06-config.md](06-config.md) | Env config + godotenv |
| [07-queue-abstraction.md](07-queue-abstraction.md) | `Enqueuer` / `Consumer`, `MemoryQueue` |
| [08-queue-from-config.md](08-queue-from-config.md) | `NewFromConfig`, `QUEUE_BACKEND` |
| [09-parse-stripe-event.md](09-parse-stripe-event.md) | `ParseStripeEvent`, fixture tests |

**Note:** There is **no `04-` doc**; work jumped to **05** for domain types (see [03 next branch](03-s3-backend-ci.md)).
