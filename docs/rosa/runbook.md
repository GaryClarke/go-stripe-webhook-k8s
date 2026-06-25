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
| **`rosa` token expired** | **`rosa login --use-auth-code`** (browser SSO) - see [Red Hat CLI login](#red-hat-cli-login-rosa) |
| **`oc` not logged in** | `oc login https://api.gc-rosa-lab.upgg.p1.openshiftapps.com:6443 -u garyc -p '<password>'` then `oc project go-stripe-webhook` |
| **`ImagePullBackOff`** | `make lab-ecr-refresh` (needs `aws` CLI) |
| **Webhook 400 / new Stripe destination** | Update `stripe-webhook-secret` with Dashboard **`whsec`** (not `stripe listen`), then `oc rollout restart deployment/go-stripe-webhook-k8s` |

**Not daily:** cluster create, IDP, first `oc apply`, Stripe Dashboard endpoint setup (once per environment).

**Health URL:** `https://go-stripe-webhook-k8s-go-stripe-webhook.apps.gc-rosa-lab.upgg.p1.openshiftapps.com/readyz` (DNS suffix changes after cluster recreate - use `oc get route` or `rosa describe cluster` for the current host).

**Deploy (manual):** `oc apply -k k8s/overlays/rosa` (after `ecr-registry`, `stripe-webhook-secret`, and `database-url` exist in project `go-stripe-webhook`).

---

## Red Hat CLI login (`rosa`)

Use this when **`rosa list clusters`**, **`make lab-off`**, or **`make lab-on`** fail with token / authentication errors, or when **`make lab-status`** prints **`>>> YOU: Refresh Red Hat SSO`**.

### Preferred: browser SSO (auth code)

```bash
rosa login --use-auth-code
```

1. CLI prints **`You will now be redirected to Red Hat SSO login`** and opens the browser (or shows a URL).
2. Sign in with your Red Hat account in the browser.
3. CLI prints **`Token received successfully`** and **`Logged in as '…' on 'https://api.openshift.com'`**.

Verify:

```bash
rosa whoami
rosa list clusters
```

**Why prefer this over `rosa login` (paste token):** No copying an offline access token from the console; fewer mistakes; same SSO session you use in the browser. **Do not paste tokens into chat** or commit them.

### Plain `rosa login` (offline token)

`rosa login` alone prompts for an **offline access token** from [console.redhat.com/openshift/token/rosa](https://console.redhat.com/openshift/token/rosa). Use only if **`--use-auth-code`** is unavailable (headless environment, browser blocked). Paste the token **only in your local terminal**.

### Switch Red Hat account

```bash
rosa logout
# Log out of https://sso.redhat.com in the browser if needed
rosa login --use-auth-code
```

### CI does not use `rosa login`

**Deploy ROSA** in GitHub Actions uses **`oc login`** with **`ROSA_API_URL`** + **`OC_LAB_PASSWORD`** (htpasswd). Local **`rosa login`** is for **`rosa`** / **`make lab-on`** / **`make lab-off`** only.

---

## Lab on / lab off

Hibernate / **`rosa stop`** is not available on this account. Use **delete / recreate** to stop billing.

| When | Command |
|------|---------|
| **End of day** | `make lab-off` (deletes cluster; ~10-20 min uninstall). Requires **`rosa login --use-auth-code`** first - script exits with instructions if not logged in. |
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
- **Deploy steps:** sync **`ecr-registry`** + **`stripe-webhook-secret`** + **`database-url`**, run Goose **`Job`** in-cluster (private RDS is not reachable from GHA), **`oc apply -k k8s/overlays/rosa`**, **`oc set image`** to **`github.sha`**, smoke **`/readyz`**.
- **GitHub:** see **GitHub Actions configuration** below.

---

## GitHub Actions configuration

Set under repo **Settings → Secrets and variables → Actions**. **Never commit values.**

**CI** (`ci.yaml`) uses **Variables** + OIDC for ECR push. **Deploy ROSA** (`deploy-rosa.yaml`) uses **Variables** + **Secrets** for `oc login` and cluster Secrets. We use **htpasswd + `ROSA_API_URL`** in CI (no Red Hat offline token).

### Variables (Actions → **Variables**)

| Variable | Purpose |
|----------|---------|
| **`AWS_ROLE_ARN`** | OIDC trust → IAM role for ECR push (from **`infra/terraform/`** output **`github_actions_role_arn`**) |
| **`AWS_REGION`** | e.g. **`eu-west-1`** (optional in workflow; defaults to **`eu-west-1`**) |
| **`ECR_REPOSITORY`** | e.g. **`go-stripe-webhook-k8s`** (optional; workflow default matches repo) |
| **`ROSA_API_URL`** | Cluster API URL for CI **`oc login`** — copy from **`make lab-on`** or **`rosa describe cluster`**; **update after every delete/recreate**; must be a **Variable** (not a Secret); **no spaces** |

Example **`ROSA_API_URL`** (suffix changes when cluster is recreated):

```text
https://api.gc-rosa-lab.upgg.p1.openshiftapps.com:6443
```

### Secrets (Actions → **Secrets**)

| Secret | Purpose |
|--------|---------|
| **`OC_LAB_PASSWORD`** | htpasswd password for user **`garyc`** (CI **`oc login`**) |
| **`STRIPE_WEBHOOK_SECRET`** | Stripe Dashboard **`whsec_...`** for the ROSA webhook endpoint (not **`stripe listen`**) |
| **`DATABASE_URL`** | Postgres DSN for idempotency ledger (Phase 9 RDS URL; synced to **`database-url`** Secret) |

### Common CI deploy mistakes

| Mistake | Symptom |
|---------|---------|
| **`ROSA_API_URL` in Secrets** instead of Variables | Deploy skips: *ROSA_API_URL not set* |
| Stale URL after **`make lab-off`** / recreate | Deploy skips: *oc login failed* or *no such host* |
| Space in URL (e.g. after `.`) | *invalid character " " in host name* |
| Cluster off | Deploy skips (expected); job stays green |

**Done gate (Phase 2):** existing Terraform GitHub OIDC role pushes ECR; deploy workflow uses same OIDC for **`aws ecr get-login-password`** plus **`oc`** credentials above.

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
| CI deploy skipped | Cluster **off** — **`make lab-on`**; or fix **`ROSA_API_URL`** (Variable, no spaces, current API from **`make lab-on`**) |
| Webhook **400** | **`stripe-webhook-secret`** matches **Dashboard** destination, not **`stripe listen`** |
