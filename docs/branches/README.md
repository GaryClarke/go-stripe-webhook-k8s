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
| `4-webhook-stripe-stub` | [04-webhook-stripe-stub.md](04-webhook-stripe-stub.md) | `POST /webhooks/stripe`, handlers + mux wiring, `ParseStripeEvent`, `apiHandler` tests |
| `5-add-dbg-dd` | [05-add-dbg-dd.md](05-add-dbg-dd.md) | `internal/dbg.DD`, `go-spew`, `//go:build debug` / `!debug` split |
| `7-api-application-wire` | [07-api-application-wire.md](07-api-application-wire.md) | `App` + `NewApp`, `routes()` on `ServeMux`, `handlers.go`, `Recover` in `main` and `apiHandler` tests |
| `8-stripe-webhook-verify` | [08-stripe-webhook-verify.md](08-stripe-webhook-verify.md) | `stripe.ConstructEvent` on raw body + `Stripe-Signature`; **400** on failure; signed payload tests |
| `9-containerise` | [09-containerise.md](09-containerise.md) | `Dockerfile` + `.dockerignore`, distroless non-root image; PLAN/README Milestone 3 |
| `10-terraform-ecr-github-oidc` | [10-terraform-ecr-github-oidc.md](10-terraform-ecr-github-oidc.md) | `infra/terraform/`: ECR, GitHub OIDC + IAM role for CI ECR push; remote state; drop legacy `terraform/` |
| `11-gh-actions-ecr-push` | [11-gh-actions-ecr-push.md](11-gh-actions-ecr-push.md) | CI: OIDC to AWS, buildx, push to ECR (`github.sha` + `latest`); Go jobs unchanged as gates |
| `12-k8s-first-deploy` | [12-k8s-first-deploy.md](12-k8s-first-deploy.md) | `k8s/`: Deployment + ClusterIP Service, ECR pull + Stripe Secrets via `kubectl`, port-forward probes |
| `13-openshift-route` | [13-openshift-route.md](13-openshift-route.md) | **`openshift/route.yaml`** (portable **Route**, TLS edge); Sandbox runbook **`docs/openshift/`** |
| `14-structured-logging` | [14-structured-logging.md](14-structured-logging.md) | **Milestone 6** (done): JSON **`slog`**, **`RequestLog`**, **`request_id`**, **`logmsg`** contract; optional polish in **PLAN** stretch |
| `15-rosa-lab-deploy` | [15-rosa-lab-deploy.md](15-rosa-lab-deploy.md) | **Milestone 7** (in progress on **`main`**): ROSA lab, kustomize, **GHA** deploy, **`lab-on`/`lab-off`**; Phase 5 test pending |
| `16-idempotency-postgres` | [16-idempotency-postgres.md](16-idempotency-postgres.md) | **Milestone 8** (in progress): Postgres **`processed_events`**, insert-on-conflict, **`/readyz`** DB check, RDS, **`replicas: 2+`** |

**Doc template:** mirror the style in [original-branches](../original-branches/README.md): goal, scope, key files, how to verify, follow-ups.

## Cross-links

- [docs/README.md](../README.md) — docs index  
- [docs/PROJECT_KNOWLEDGE.md](../PROJECT_KNOWLEDGE.md) — historical Lambda architecture and original build order §5  
