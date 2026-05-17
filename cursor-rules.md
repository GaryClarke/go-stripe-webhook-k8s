# Cursor rules for Integration Engine

Use this file as guardrails for AI and humans working in this repo. Keep it updated as we add conventions.

## Documentation

- **No em dashes.** Use a hyphen with spaces (" - ") or rephrase. Do not use the em dash character (Unicode U+2014) in docs, READMEs, or comments.
- **K8s / HTTP track:** Document each numbered unit of work in `docs/branches/` (see `docs/branches/README.md`). **Lambda-era** branch logs live in `docs/original-branches/`. A matching **`docs/branches/NN-*.md`** and index row are **required before merging a numbered branch to `main`** (see **`mmp`** in Commits and branches §).
- Update `docs/PROJECT_KNOWLEDGE.md` only when locking Lambda-era architecture or cross-cutting process. The K8s roadmap is [PLAN.md](PLAN.md).
- **Commands / chat shorthands** (**aac**, **ms**, **mmp**, etc.) are defined **only in this file** (Commits and branches §). Do not copy their definitions into [PLAN.md](PLAN.md), [docs/branches/README.md](docs/branches/README.md), or elsewhere - link here instead.

## Code and layout

- **Go:** Idiomatic, production-style. Prefer standard library; add deps only when needed. Use `internal/` for shared packages, `cmd/` for entry points.
- **Naming:** Match existing style in the file. Use clear, domain-focused names (e.g. `HandleWebhook`, HTTP handlers).
- **Tests:** Table-driven where it fits. Use fixtures in `testdata/`. Prefer interfaces for anything we might swap (e.g. downstream client, event publisher).
- **Config:** Env-based. No hardcoded secrets or URLs. Validate required vars at startup.

## Architecture

- **Entry point:** **`cmd/api`** HTTP server on Kubernetes (local binary and container). Shared logic lives in **`internal/`**; **`cmd/`** only wires and invokes.
- **Events:** A later milestone introduces **Kafka** between ingestion and processing; this repo does not ship SQS or an in-process queue package until that work lands.
- **Stripe:** Support the event shapes we document (v1: **`invoice.payment_succeeded`**, **`invoice.payment_failed`**). Validate and normalise at the webhook boundary; downstream consumers stay agnostic of Stripe where possible.

## Commits and branches

- Work in **small numbered branches** (e.g. `1-add-livez-and-readyz`). One logical learning unit per branch. Add a matching write-up `docs/branches/NN-descriptive-name.md` when the branch lands (**required before `mmp`**; see below).
- Keep `main` buildable and tested.
- **aac** = add and commit. Shorthand for `git add -A && git commit` (stage all changes and commit). Use when the user says "aac" or "aac with message '...'". **Before** **aac**, if the working tree or staged diff touches **`infra/terraform/`** (or the conversation is clearly about that Terraform root), run the **Terraform apply check** in **Infra - Terraform apply gate (aac / mmp)** below so the user does not commit while AWS is still out of date.
- **ms** = branch / milestone complete. When the user says `ms <branch-name>` (e.g. `ms 1-add-livez-and-readyz`), treat that branch as **done**: update `docs/branches/` (index row, new `NN-*.md` if needed), align `README.md` / `PLAN.md` if needed, and prep next steps. That write-up and index link are **required** before **`mmp`** can merge the branch to **`main`**. **Does not** by itself imply git merge or push - use **mmp** when they want integration to `origin/main`. If the branch touched **`infra/terraform/`**, mention that **`mmp`** (or **aac** before it) should still run the **Terraform apply gate** so AWS matches the code.
- **mmp** = merge to `main` and push (**m**erge **m**ain **p**ush). When the user says **`mmp`** with **no** branch name: use **current branch** (must not be `main`). **First** check `git status`. If there are **uncommitted changes**, **do not** merge yet: give a **short summary** of what changed (files + intent), say whether you **recommend committing as-is** (yes/no + one-line reason), and suggest a **proposed commit message**; wait for the user to **aac** (or adjust) before continuing. If the tree is **clean**, **then** confirm the **branch write-up exists** before merging: the branch name must start with a numeric segment (e.g. **`7-api-application-wire`** → **`7`**). There must be a file **`docs/branches/NN-*.md`** whose **`NN`** is that number **zero-padded to two digits** (e.g. **`07-api-application-wire.md`**). **`docs/branches/README.md`** must list that branch with a link to that file. If the doc or index row is missing, **do not merge**: run **`ms <branch-name>`** (or add the write-up and table row), **aac**, then **`mmp`**. **Before** merge (clean tree and docs OK), if the branch changes **`infra/terraform/`** or Milestone 3 infra is in scope, run the **Terraform apply check** in **Infra - Terraform apply gate (aac / mmp)** below. If **apply is still required** in AWS, **do not merge** until the user confirms infra is applied or explicitly accepts merging code-only. After that check passes, `git checkout main`, `git pull` (respect ff-only if configured), `git merge <that-branch>`, `git push origin main`. When they say **`mmp <branch-name>`**, same idea: dirty tree → pause; clean tree → **branch write-up check** → **Terraform apply gate when relevant** → merge; `push origin main`. On merge conflicts, stop and surface conflict files - do not force-push to `main`. No squash merge unless they ask.

## Infra (when we add it)

- **Kubernetes:** Manifests under **`k8s/`** (and **`openshift/`** for routes). See **[PLAN.md](PLAN.md)**.
- **Legacy `terraform/`** (Lambda + API Gateway from the parent project) is **removed** from this fork once **`10-terraform-ecr-github-oidc`** lands; new **IaC** for **ECR** + **GitHub OIDC** lives under an agreed path (e.g. **`infra/terraform/`**). See **Milestone 3** in **[PLAN.md](PLAN.md)**.

### Terraform apply gate (**aac** / **mmp**)

Use this when **`infra/terraform/`** changed on the branch, or when **aac** / **mmp** is happening on an infra-focused branch, so AWS does not lag the Git repo.

1. **Working directory:** `infra/terraform` (from repo root: `cd infra/terraform`).
2. **Init:** Run **`terraform init`** when needed (first time, backend change, or provider lock update). If **init** fails (no AWS access to S3 backend, etc.), **stop**: tell the user to run the same commands locally with credentials, then repeat the gate.
3. **Pending changes:** Run **`terraform plan -detailed-exitcode`** (or **`terraform plan`** and interpret the summary). With **`-detailed-exitcode`**: **0** = no changes; **2** = changes pending (**apply** needed); **1** = error (**do not** treat as clean).
4. **No state / first-time:** If **`terraform state list`** reports **no state file** or an **empty** list while **`.tf`** defines resources, treat as **apply needed** (bootstrap).
5. **User step (required):** If **apply is needed**, **never** run **`terraform apply`** without **explicit confirmation**. **Either**
   - **ask:** "Terraform shows pending changes for AWS. Run **`terraform apply`** now? **(y/n)**" and wait for **y**, **or**
   - **advise:** summarize what would change in one short line, then give copy-paste commands, e.g. `cd infra/terraform && terraform plan && terraform apply`.
   Prefer **y/n** when the user is in an interactive chat; use **advise** if they are clearly batching instructions or have declined auto-runs before.
6. **After apply:** Remind them **`terraform output`** (e.g. **`github_actions_role_arn`**) is available only once state exists **after** a successful apply.
