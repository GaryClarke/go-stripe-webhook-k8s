# Branch 12: First Kubernetes deploy (`12-k8s-first-deploy`)

**Goal:** **[PLAN.md](../../PLAN.md) Milestone 4** (**Phase A**): run the **`cmd/api`** image from **ECR** on a **local** cluster (**Docker Desktop** Kubernetes): **Deployment** (**probes**, **`securityContext`**, **resources**, **`imagePullSecrets`**), **ClusterIP Service**, **port-forward**, **`curl`** **`/livez`** and **`/readyz`**. **Secrets** are created with **`kubectl`** (no real Stripe values in git).

## What we added

- **`k8s/deployment.yaml`**: **ECR** image, **`imagePullSecrets: ecr-registry`**, **`STRIPE_WEBHOOK_SECRET`** from **`stripe-webhook-secret`**, **`/readyz`** / **`/livez`** probes, **`runAsNonRoot`** + **`seccompProfile RuntimeDefault`**, CPU/memory requests and memory limit.
- **`k8s/service.yaml`**: **`ClusterIP`**, **`port` / `targetPort` `8080`**, **`targetPort: http`** (named port on the container).

## Prerequisites (cluster-local, not in git)

Before **`kubectl apply -f k8s/`**:

1. **ECR pull Secret** (**tokens expire**, recreate when pulls fail **`ImagePullBackOff`**):

   ```bash
   kubectl delete secret ecr-registry -n default --ignore-not-found
   kubectl create secret docker-registry ecr-registry \
     --docker-server=<ACCOUNT>.dkr.ecr.<REGION>.amazonaws.com \
     --docker-username=AWS \
     --docker-password=$(aws ecr get-login-password --region <REGION>) \
     --namespace=default
   ```

2. **Stripe signing secret placeholder** (example for learning):

   ```bash
   kubectl create secret generic stripe-webhook-secret \
     --from-literal=STRIPE_WEBHOOK_SECRET='whsec_test' \
     --namespace=default
   ```

Use a real **`whsec_...`** from Stripe only outside learning sandboxes — **never** commit it.

## Files changed (high level)

- **`k8s/deployment.yaml`**, **`k8s/service.yaml`**

## How to verify

```bash
kubectl apply -f k8s/
kubectl get pods -l app=go-stripe-webhook-k8s -n default
kubectl get svc,ep -l app=go-stripe-webhook-k8s -n default
kubectl port-forward svc/go-stripe-webhook-k8s 8080:8080 -n default
curl -sS http://127.0.0.1:8080/livez
curl -sS http://127.0.0.1:8080/readyz
```

Expect **`READY 1/1`** and **`{"status":"ok"}`** on both probes.

## Follow-ups

- **PLAN Milestone 5 (external access):**
  - **OpenShift Route** (**`openshift/route.yaml`**) — align with Barclays / OpenShift-style edge exposure (**TLS**, **DNS** controlled by the platform).
  - **Kubernetes Ingress** (generic counterpart) — same traffic story (`internet → controller → Ingress → Service → Pods`) on plain **Kubernetes**.
- **`PLAN` optional stretch:** **`replicas: 3`** to see Service distribution; **immutable image tags** + deploy automation (**CI** patching image or **`kubectl set image`**).
- Optional YAML: **`allowPrivilegeEscalation: false`**, **`readOnlyRootFilesystem`** when the distroless runtime allows.
