# ROSA lab runbook

Operational guide for **Phase C** ([**PLAN.md**](../../PLAN.md) Milestone 7). **Build order** lives in **[docs/branches/15-rosa-lab-deploy.md](../branches/15-rosa-lab-deploy.md)** — this file is for day-two ops once implemented.

**Chat shorthand:** **`start work`** — see **[cursor-rules.md](../../cursor-rules.md)** (agent runs **`make lab-status`**; you only do interactive auth / secrets when prompted).

---

## Session startup (typical workday)

**One command (agent or you):**

```bash
make lab-status
```

**You only run these when `lab-status` says `>>> YOU:`**

| Situation | Command |
|-----------|---------|
| **`rosa` token expired** | `rosa login --use-auth-code` |
| **`oc` not logged in** | `oc login https://api.gc-rosa-lab.bd91.p1.openshiftapps.com:6443 -u garyc -p '<password>'` then `oc project go-stripe-webhook` |
| **`ImagePullBackOff`** | `make lab-ecr-refresh` (needs `aws` CLI) |
| **Webhook 400 / new Stripe destination** | Update `stripe-webhook-secret` with Dashboard **`whsec`** (not `stripe listen`), then `oc rollout restart deployment/go-stripe-webhook-k8s` |

**Not daily:** cluster create, IDP, first `oc apply`, Stripe Dashboard endpoint setup (once per environment).

**Health URL:** `https://go-stripe-webhook-k8s-go-stripe-webhook.apps.gc-rosa-lab.bd91.p1.openshiftapps.com/readyz`

---

## Lab on / lab off

**Prefer stop/start** over delete/recreate (faster turn-around).

```bash
# Planned: make lab-off / make lab-on (scripts not yet added)
# Today: console may only offer Delete; rosa stop unavailable on some CLI versions.
# Leaving cluster "ready" overnight is OK for short lab use; use Delete only for long breaks.
```

When the cluster is **stopped**, **GitHub Actions deploy** should **skip** (not fail). Turn the lab **on** before merging to **`main`** if you need that push deployed immediately.

**Terraform:** **`lab_enabled=false`** is for longer breaks (optional destroy path — document in **`infra/terraform/rosa/README.md`** once added).

---

## CI deploy (automatic)

- **Trigger:** every **`push` to `main`** (after image push to ECR).
- **No** manual **`workflow_dispatch`** for normal deploys.
- Deploy job: login **`oc`**, sync Secrets, apply **`k8s/`** + **`openshift/route.yaml`**, set image to **`github.sha`**, smoke **`/readyz`**.

---

## GitHub Actions secrets (expected)

Set under repo **Settings → Secrets and variables → Actions**. **Never commit values.**

| Secret | Purpose |
|--------|---------|
| *(existing vars)* **`AWS_ROLE_ARN`** | OIDC → AWS (ECR, optional TF) |
| **`ROSA_TOKEN`** (or agreed name) | **`oc login`** / **`rosa`** API from CI |
| **`STRIPE_WEBHOOK_SECRET`** | Dashboard signing secret for ROSA endpoint |

---

## Manual verify (smoke)

```bash
oc project <your-project>
oc get route go-stripe-webhook-k8s -o jsonpath='https://{.spec.host}{"\n"}'
curl -fsS "https://<route-host>/readyz"
oc logs -l app=go-stripe-webhook-k8s --tail=50
```

Stripe: point Dashboard webhook at **`https://<route-host>/webhooks/stripe`**; use that destination's **`whsec`**.

---

## Cost notes (fill in after Phase 1)

| State | Rough cost | Notes |
|-------|------------|--------|
| Cluster **running** | _TBD $/day_ | Workers + ROSA control-plane fee |
| Cluster **stopped** | _TBD $/day_ | Lower; some control-plane cost may remain |

Update this table after first **`terraform apply`** / billing review.

---

## Troubleshoot

| Symptom | Check |
|---------|--------|
| **`ImagePullBackOff`** | **`ecr-registry`** Secret; CI refresh on deploy |
| Route **503** / app unavailable | **`oc get pods`** Ready; **`oc describe route`** |
| CI deploy skipped | Cluster **stopped** — **`make lab-on`** |
| Webhook **400** | **`stripe-webhook-secret`** matches **Dashboard** destination, not **`stripe listen`** |
