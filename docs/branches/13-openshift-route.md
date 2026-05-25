# Branch 13: OpenShift routing (`13-openshift-route`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 5**: add a **`Route`** that exposes **`k8s/service.yaml`** (**`ClusterIP`** **`go-stripe-webhook-k8s`**) **outside** the cluster with **HTTPS** (**TLS **`edge`** at the router**) and **`http`** → **`https`** redirect. **Phase B** uses the same **`k8s/`** manifests on the [OpenShift Developer Sandbox](https://developers.redhat.com/developer-sandbox).

## What we added

- **`openshift/route.yaml`**: Minimal, **portable** **`Route`** (no **`spec.host`**, no **`metadata.namespace`** in-file so **`oc project`** / **`-n`** decide context). **`targetPort: http`** matches the **named** Service port (**`8080`** on the Pods). **`tls.termination: edge`**, **`insecureEdgeTerminationPolicy: Redirect`**.
- **`docs/openshift/sandbox-runbook.md`**, **`docs/openshift/README.md`**: Sandbox **`oc`** flows (login, refresh **ECR** **`docker-registry`** Secret when **`ImagePullBackOff`**), **`oc apply`** **`k8s/`**, **`oc apply`** or console **Route**, **`curl`** checks, **`Application is not available`** troubleshooting.
- **Root `README.md`**: Layout row for **`openshift/`** linked to **`docs/openshift/sandbox-runbook.md`**.

## Prerequisites (Sandbox, not in git)

Secrets and cluster state match **[docs/branches/12-k8s-first-deploy.md](12-k8s-first-deploy.md)** but with **`oc`** (**`stripe-webhook-secret`**, **`ecr-registry`** in the target project).

## Files changed (high level)

- **`openshift/route.yaml`**
- **`docs/openshift/*.md`**
- **`README.md`** (Layout)
- **`PLAN.md`** (Milestone 5 status)
- **`docs/branches/README.md`**, **`docs/branches/12-k8s-first-deploy.md`** (follow-ups pointer)

## How to verify

In the Sandbox project (**`oc apply`** **`k8s/`** first; Pods **READY**):

```bash
oc apply -f openshift/route.yaml
HOST=$(oc get route go-stripe-webhook-k8s -o jsonpath='{.spec.host}{"\n"}')
curl -sS "https://${HOST}/livez"
curl -sS "https://${HOST}/readyz"
```

Expect **`{"status":"ok"}`** on both probes (JSON from **`cmd/api`**, not router HTML).

## Follow-ups

- **Optional:** Real **Stripe** webhook signing secret (**`stripe-webhook-secret`**) and **HTTPS** webhook URL (**`/webhooks/stripe`**) toward **Phase B** before **Milestone 6**.
- **Stretch:** **`oc apply -f openshift/route.yaml`** (and **`k8s/`**) from **CI** plus **automated Secrets** (**[PLAN.md](../../PLAN.md)** Stretch - deploy + secret automation).
