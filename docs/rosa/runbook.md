# ROSA lab runbook

Operational guide for **Phase C** ([**PLAN.md**](../../PLAN.md) Milestone 7). **Build order** lives in **[docs/branches/15-rosa-lab-deploy.md](../branches/15-rosa-lab-deploy.md)** — this file is for day-two ops once implemented.

---

## Lab on / lab off

**Prefer stop/start** over delete/recreate (faster turn-around).

```bash
# Stop workers — lower cost; Route/API may be unavailable until start
make lab-off    # or: ./scripts/rosa-lab-off.sh

# Start again before learning or before expecting CI deploy to run
make lab-on     # or: ./scripts/rosa-lab-on.sh
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
