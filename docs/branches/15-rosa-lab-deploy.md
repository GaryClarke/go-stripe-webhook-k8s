# Branch 15: ROSA lab deploy (`15-rosa-lab-deploy`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 7** — **Phase C**: deploy **go-stripe-webhook-k8s** on **ROSA** in your **AWS** account; public **HTTPS** via **`openshift/route.yaml`**; **lab cost on/off** documented and scripted.

## Learn

- **ROSA** cluster lifecycle in **your** AWS account (Barclays-aligned **OpenShift on AWS**).
- Reuse **`k8s/`** + **`Route`** from **M4–M5**; **`oc`** deploy and logs on a real cluster.
- Minimal **CI deploy** (optional **`workflow_dispatch`**).
- **Cost control:** **`rosa stop cluster`** / **`rosa start cluster`** (or agreed equivalent) — not 24/7.

## Build (planned)

| Area | Notes |
|------|--------|
| **Cluster** | **`rosa` CLI** and/or **`infra/terraform/`** (decide in branch; document in runbook) |
| **Runbook** | **`docs/rosa/`** or extend **`docs/openshift/`** — primary env for **M7+** |
| **Scripts** | e.g. **`scripts/rosa-lab-start.sh`**, **`scripts/rosa-lab-stop.sh`** or **Makefile** targets |
| **Manifests** | Existing **`k8s/deployment.yaml`**, **`k8s/service.yaml`**, **`openshift/route.yaml`** |
| **Secrets** | **`ecr-registry`**, **`stripe-webhook-secret`** on cluster (documented **`oc create`**) |
| **CI** | Optional GHA job: deploy image tag to ROSA (**kubeconfig** / token in **GH Secrets**) |

## Out of scope

- **RDS** / **Postgres** idempotency (**Milestone 8**)
- **Kafka** (**Milestone 9**)
- Full **External Secrets Operator** (PLAN **Stretch**)

## Done when

1. **`curl https://<route-host>/readyz`** returns **`cmd/api`** JSON from the internet.
2. Webhook delivery to the **ROSA** URL shows structured logs in **`oc logs`**.
3. **Lab off** and **lab on** are repeatable via documented scripts (target: short command sequence, not cluster recreate every time).

## Verify (checklist)

- [ ] ROSA cluster created in learning AWS account
- [ ] **`oc apply -f k8s/`** + **`oc apply -f openshift/route.yaml`**
- [ ] ECR image pull works (**`ecr-registry`** Secret)
- [ ] Stripe signing secret on cluster
- [ ] Public **`/readyz`** and test webhook
- [ ] **`rosa stop`** / **`rosa start`** (or documented alternative) tested
- [ ] Runbook lists expected **$/day** when lab is up vs stopped

## Follow-ups

- **Milestone 8:** Terraform **RDS** + **`processed_events`** + **`replicas: 2+`**
- **Stretch:** automated ECR pull secret rotation; **External Secrets**
