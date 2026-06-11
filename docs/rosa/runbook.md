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
| **`oc` not logged in** | `oc login https://api.gc-rosa-lab.upgg.p1.openshiftapps.com:6443 -u garyc -p '<password>'` then `oc project go-stripe-webhook` |
| **`ImagePullBackOff`** | `make lab-ecr-refresh` (needs `aws` CLI) |
| **Webhook 400 / new Stripe destination** | Update `stripe-webhook-secret` with Dashboard **`whsec`** (not `stripe listen`), then `oc rollout restart deployment/go-stripe-webhook-k8s` |

**Not daily:** cluster create, IDP, first `oc apply`, Stripe Dashboard endpoint setup (once per environment).

**Health URL:** `https://go-stripe-webhook-k8s-go-stripe-webhook.apps.gc-rosa-lab.upgg.p1.openshiftapps.com/readyz` (DNS suffix changes after cluster recreate - use `oc get route` or `rosa describe cluster` for the current host).

**Deploy (manual):** `oc apply -k k8s/overlays/rosa` (after `ecr-registry` and `stripe-webhook-secret` exist in project `go-stripe-webhook`).

---

## Lab on / lab off

Hibernate / **`rosa stop`** is not available on this account. Use **delete / recreate** to stop billing.

| When | Command |
|------|---------|
| **End of day** | `make lab-off` (deletes cluster; ~10-20 min uninstall) |
| **Start of day** | `make lab-on` (creates cluster if missing, or prints status + checklist) |
| **Cluster ready** | Follow checklist from `make lab-on` (GitHub **`ROSA_API_URL`**, Stripe URL, IDP/setup) |
| **Working** | `make lab-status` |

**Morning must-dos after recreate** (DNS suffix changes each time):

1. **GitHub variable `ROSA_API_URL`** — copy API URL from `rosa describe cluster` (no spaces).
2. **Stripe** — edit existing webhook endpoint URL (same `whsec` usually).
3. **Fresh cluster** — IDP, `oc new-project`, secrets (or rely on CI deploy after step 1).

When the cluster is **off**, **Deploy ROSA** on push to **`main`** **skips** (exit 0, not a failure). Turn the lab **on** and update **`ROSA_API_URL`** before you need CI deploy to land.

**Terraform:** **`lab_enabled=false`** is for longer breaks (optional destroy path — document in **`infra/terraform/rosa/README.md`** once added).

---

## CI deploy (automatic)

- **Workflow:** **`.github/workflows/deploy-rosa.yaml`** runs after **`CI`** succeeds on **`main`**.
- **Trigger:** every **`push` to `main`** (after **`push-ecr`** in **`ci.yaml`**).
- **Skips** (exit 0) when cluster is off, **`ROSA_API_URL`** unset, or **`oc login`** fails.
- **Deploy steps:** sync **`ecr-registry`** + **`stripe-webhook-secret`**, **`oc apply -k k8s/overlays/rosa`**, **`oc set image`** to **`github.sha`**, smoke **`/readyz`**.
- **GitHub:** variable **`ROSA_API_URL`**; secrets **`OC_LAB_PASSWORD`**, **`STRIPE_WEBHOOK_SECRET`**.

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
