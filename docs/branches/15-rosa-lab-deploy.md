# Branch 15: ROSA lab deploy (`15-rosa-lab-deploy`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 7** — **Phase C**: **ROSA** in your **AWS** account; app on **HTTPS Route**; **Terraform + CI** (no manual deploy triggers); **lab on/off** via IaC and scripts.

**Status:** In progress (planning documented; implementation follows phases below).

---

## Automation decisions (locked for M7)

| Decision | Choice |
|----------|--------|
| Cluster | **Terraform** (ROSA provider or agreed module) in **`infra/terraform/rosa/`** (separate state from ECR/OIDC) |
| App deploy | **GitHub Actions** on **`push` to `main`** (after **`push-ecr`**) — not **`workflow_dispatch`** |
| Manifests | Git **`k8s/`** + **`openshift/route.yaml`** applied by CI (**`oc`** or **`kubectl`**) |
| Secrets | **GitHub Actions secrets** → cluster **`Secret`** at deploy time (no values in git). ECR pull via fresh **`docker-registry`** Secret each deploy (or CronJob stretch). |
| Cost control | **`lab_enabled`** Terraform variable + **`rosa stop` / `rosa start`** wrappers; CI **skips deploy** when cluster is stopped (success, not failure) |
| Image tag | CI sets Deployment image to **`${{ github.sha }}`** (remove hardcoded ECR URL from committed YAML over time — use kustomize, envsubst, or TF) |

---

## Architecture (target)

```text
GitHub push main
  -> CI: build, test, lint, push ECR (:sha + :latest)
  -> CI: if cluster running -> apply k8s + Route, sync Secrets, rollout image
  -> Internet -> OpenShift Router (TLS edge) -> Route -> Service -> Pods

Lab off:
  rosa stop cluster  (or lab_enabled=false + documented destroy path)
Lab on:
  rosa start cluster + terraform apply (if infra drift)
```

---

## Implementation order

Work **in this sequence**. Each phase has a **done gate** before the next.

### Phase 0 — Prerequisites (one-time, partly manual)

**Learn:** ROSA billing, Red Hat ↔ AWS account link, minimum cluster size.

**Do:**

1. Red Hat account with **ROSA** enabled; link to your **AWS** account (console / **`rosa create account`**).
2. Install **`rosa`** and **`oc`** locally; **`aws` CLI** configured for learning account.
3. Confirm **AWS service quotas** (enough vCPUs for a small worker pool in **`eu-west-1`** or chosen region).
4. Note existing **ECR + GitHub OIDC** from **`infra/terraform/`** — do **not** destroy; ROSA uses a **separate** Terraform root/state.

**Done gate:** **`rosa verify`** (or equivalent) passes; you can **`aws sts get-caller-identity`**.

---

### Phase 1 — Terraform: ROSA cluster skeleton

**Learn:** ROSA cluster + machine pool as desired state; **`lab_enabled`** pattern.

**Do:**

1. Add **`infra/terraform/rosa/`** with its own **S3 backend key** (e.g. **`rosa/terraform.tfstate`**).
2. Variables: **`lab_enabled`**, **`aws_region`**, cluster name, worker **instance type** / **replica count** (minimal for lab).
3. Resources: ROSA cluster, default machine pool (when **`lab_enabled = true`**).
4. Outputs: **API URL**, **console URL**, **cluster ID**, **cluster admin** bootstrap hint (document — do not commit kubeconfig).
5. Document **`terraform apply`** with **`lab_enabled=true`** and first-time cluster create time (~30–45 min).

**Done gate:** **`oc login`** against ROSA API works; **`oc get nodes`** shows Ready workers.

**Files (expected):** **`infra/terraform/rosa/main.tf`**, **`variables.tf`**, **`outputs.tf`**, **`versions.tf`**, **`README.md`**.

---

### Phase 2 — Terraform: IAM for CI (extend or sibling module)

**Learn:** Least-privilege IAM for GHA beyond ECR push.

**Do:**

1. Extend **`infra/terraform/`** IAM **or** add roles in **`infra/terraform/rosa/`** trusted by same GitHub OIDC provider.
2. CI role needs (narrowly): **ECR push** (existing), **read ROSA/cluster** (if TF in CI), **Secrets Manager** read (if used later), **enough to run deploy job** (often cluster credentials come from **ROSA token** + **`oc login`** in CI, not broad IAM).
3. Document required **GitHub Actions variables/secrets** in **`docs/rosa/runbook.md`**.

**Done gate:** A test workflow step can **`configure-aws-credentials`** and call agreed AWS APIs without static keys.

---

### Phase 3 — Parameterise app manifests for CI

**Learn:** Same YAML locally and in cloud; image and registry not hardcoded in git.

**Do:**

1. Stop committing account-specific **ECR URL** as the only deploy path — options:
   - **`k8s/overlays/rosa/`** (kustomize image transformer), or
   - CI **`sed`/`envsubst`** on a template, or
   - Terraform **kubernetes** provider (heavier; optional defer).
2. Keep **`openshift/route.yaml`** portable (already is).
3. Add **`k8s/namespace.yaml`** or document **`oc new-project`** for ROSA project name.

**Done gate:** Local **`kubectl apply -k ...`** still works; CI can set **image: `<ecr-url>:<sha>`** without editing tracked files permanently.

---

### Phase 4 — GitHub Actions: deploy workflow

**Learn:** Push-to-main deploy; conditional job when lab is off.

**Do:**

1. Add **`.github/workflows/deploy-rosa.yaml`** (name TBD):
   - **`on: push: branches: [main]`**
   - **`needs`** or ordering: run after **`push-ecr`** succeeds (workflow dependency or single workflow with jobs).
2. Job steps (high level):
   - Checkout
   - AWS OIDC login
   - **Cluster liveness check** (e.g. **`rosa describe cluster`** state **Ready** / **Hibernating**)
   - If **Hibernating** or **lab off** → **exit 0 skip** with log message
   - **`oc login`** using **ROSA token** from **GH secret** (short-lived where possible)
   - Create/update **`ecr-registry`** pull Secret ( **`aws ecr get-login-password`** )
   - Create/update **`stripe-webhook-secret`** from **GH secret**
   - **`oc apply -f k8s/`** + **`openshift/route.yaml`**
   - **`oc set image deployment/... api=<ecr>:<sha>`** or apply kustomize overlay
   - **`oc rollout status`**
   - Smoke: **`curl -fsS https://$(oc get route ... -o jsonpath=...)/readyz`**
3. Restrict deploy to **`main`** only (same as ECR).

**Done gate:** Push to **`main`** rolls out new image to ROSA without manual **`oc`** on your laptop.

---

### Phase 5 — Lab cost control

**Learn:** Stop/start vs destroy; CI behaviour when lab is off.

**Do:**

1. **`scripts/rosa-lab-on.sh`** — **`rosa start cluster`**, wait Ready, optional **`cd infra/terraform/rosa && terraform apply -var=lab_enabled=true`**
2. **`scripts/rosa-lab-off.sh`** — **`rosa stop cluster`** (preferred for speed) **or** **`lab_enabled=false`** apply for longer breaks
3. **`Makefile`** targets **`lab-on`** / **`lab-off`** wrapping scripts
4. Document in **`docs/rosa/runbook.md`**:
   - Rough **$/day** cluster **running** vs **stopped**
   - Rule: **lab off** when not actively learning
   - CI **skips** deploy when stopped (expected)

**Done gate:** You can run **`make lab-off`**, confirm deploy workflow **skips**, then **`make lab-on`**, push a commit, and deploy **succeeds**.

---

### Phase 6 — End-to-end verification

**Do:**

1. **`curl https://<route-host>/readyz`** → **`cmd/api`** JSON
2. Stripe Dashboard webhook → ROSA Route URL (Dashboard **`whsec`**, not CLI listen secret)
3. **`oc logs -l app=go-stripe-webhook-k8s`** → structured JSON, **`request_id`**, **`event_id`**
4. Runbook walkthrough without Sandbox

**Done gate:** **[PLAN.md](../../PLAN.md) Milestone 7 done when** satisfied.

---

## Out of scope (M7)

- **RDS** / idempotency (**Milestone 8**)
- **Kafka** (**Milestone 9**)
- **External Secrets Operator** (stretch; GH secrets → **`oc create`** in CI is enough for M7)
- **Multi-replica** idempotency proof (**M8**)

---

## Key files (existing vs new)

| Existing | New / changed (planned) |
|----------|-------------------------|
| **`k8s/deployment.yaml`**, **`service.yaml`** | Parameterised image; maybe **`overlays/rosa`** |
| **`openshift/route.yaml`** | Unchanged |
| **`infra/terraform/`** (ECR, OIDC) | Unchanged state |
| **`.github/workflows/ci.yaml`** | Maybe merge deploy job or keep separate workflow |
| | **`infra/terraform/rosa/`** |
| | **`.github/workflows/deploy-rosa.yaml`** |
| | **`scripts/rosa-lab-on.sh`**, **`scripts/rosa-lab-off.sh`** |
| | **`docs/rosa/runbook.md`** |

---

## Verify checklist

- [ ] Phase 0: ROSA + AWS linked
- [ ] Phase 1: Terraform creates cluster; **`oc get nodes`**
- [ ] Phase 2: CI AWS/OIDC permissions documented
- [ ] Phase 3: Image deployable without hardcoded SHA in git
- [ ] Phase 4: Push **`main`** deploys automatically
- [ ] Phase 5: **`lab-on`** / **`lab-off`** + CI skip when stopped
- [ ] Phase 6: Public **`/readyz`** + Stripe webhook + **`oc logs`**

---

## Follow-ups

- **Milestone 8:** Terraform **RDS** in same VPC; **`processed_events`**; **`replicas: 2+`**
- **Stretch:** ECR pull Secret rotation; **External Secrets**; **Sealed Secrets**
