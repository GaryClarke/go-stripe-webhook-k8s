# OpenShift Developer Sandbox - project runbook

Commands and flows for deploying **go-stripe-webhook-k8s** on **[OpenShift Developer Sandbox](https://developers.redhat.com/developer-sandbox)** using the same **`k8s/`** manifests as local Kubernetes. Concepts transfer to employer clusters (**OpenShift**) even when exact commands (**`oc`** vs **`kubectl`**, RBAC, quotas) differ.

**Related:** roadmap **[PLAN.md](../../PLAN.md)** Milestone **5** (routing); Stretch **deploy + secret automation** for pipeline / operator patterns later.

---

## 1. CLI and login

Install **`oc`**, then paste **Copy login command** from the Sandbox **web console** (user menu):

```bash
oc login --token=sha256~<token> --server=https://api.<sandbox-cluster>.openshiftapps.com:6443
```

**Token expiry:** if **`the server has asked for the client to provide credentials`** or **`You must be logged in`**, regenerate the login command from the console and **`oc login`** again. **Never** commit tokens or paste them into **`git`**.

Pick your **project** (Sandbox usually gives **`<username>-dev`**):

```bash
oc project YOUR-PROJECT-dev
oc whoami
```

---

## 2. Registry pull credentials (Private ECR)

ECR passwords are short-lived (**~12h**); recreate **`ecr-registry`** when **`ImagePullBackOff`** appears.

Replace **`<ACCOUNT>`**, **`<REGION>`**, and **`YOUR-PROJECT-dev`** as needed (**`-n`** optional when **`oc project`** matches):

```bash
oc create secret docker-registry ecr-registry \
  --docker-server=<ACCOUNT>.dkr.ecr.<REGION>.amazonaws.com \
  --docker-username=AWS \
  --docker-password=$(aws ecr get-login-password --region <REGION>)

oc secrets link default ecr-registry --for=pull
```

**Alternatively:** **`imagePullSecrets: - name: ecr-registry`** on the **Deployment** (see **`k8s/deployment.yaml`**) achieves the same for that workload **without **`secrets link`** for **`default`** (both are valid patterns).

Confirm **AWS CLI** credentials on **your laptop** (**`aws sts get-caller-identity`**).

---

## 3. Stripe signing secret (**Kubernetes **`Secret`**)

**Learning placeholder** (**do not commit** real **`whsec_…`**):

```bash
oc create secret generic stripe-webhook-secret \
  --from-literal=STRIPE_WEBHOOK_SECRET='whsec_test'
```

For **Stripe → public Route**, replace with your **Stripe Dashboard** webhook signing secret **only inside the cluster**.

---

## 4. Deploy workload (**same **`YAML`** as local)

From repo root (paths match **`main`**):

```bash
oc apply -f k8s/deployment.yaml
oc apply -f k8s/service.yaml
```

Check:

```bash
oc get pods -l app=go-stripe-webhook-k8s
oc logs -l app=go-stripe-webhook-k8s --tail=30
oc get svc go-stripe-webhook-k8s
oc get endpoints go-stripe-webhook-k8s
```

Older clusters may warn that **`Endpoints`** is deprecated in favour **`EndpointSlice`** - backends are healthy if **`Endpoints`** or **`endpointslice`** show **Pod IPs** on port **8080**:

```bash
oc get endpointslice -l kubernetes.io/service-name=go-stripe-webhook-k8s
```

---

## 5. **`Route`** (public HTTPS → **ClusterIP **`Service`)

**Console:** Networking → Routes → Create ( **Service **`go-stripe-webhook-k8s`**, **port** **8080**, **TLS** edge, **redirect** insecure traffic). Leave **hostname** blank to accept the generated **`*.apps.*`** hostname unless you manage custom DNS/certs.

**CLI** (minimal edge route):

```bash
oc create route edge go-stripe-webhook-k8s \
  --service=go-stripe-webhook-k8s \
  --port=8080
```

Inspect:

```bash
oc get routes
oc get route go-stripe-webhook-k8s -o jsonpath='{.spec.host}{"\n"}'
```

The **`jsonpath`** example prints **`spec.host`** plus a newline (no full JSON dump).

**Certificates:** Sandbox uses the **cluster router default certificate** unless you attach your own (**custom domain** scenarios).

Smoke test (**replace host**):

```bash
curl -sS https://<route-host>/livez
curl -sS https://<route-host>/readyz
```

Then configure **Stripe webhook URL** (**`https://<route-host>/webhooks/stripe`**) when using a **real** signing secret.

### If **`curl`** returns HTML (**Application is not available**)

That page is from the **OpenShift router**, not your **`cmd/api`** JSON. It usually means **no ready backend** for this **Route**, or the **Route** does not match the **Service**/**port**/**path** you expect.

**Checklist** (same **project** as the **Route** - **`oc project`**):

```bash
oc get pods -l app=go-stripe-webhook-k8s
oc get endpoints go-stripe-webhook-k8s
oc describe route go-stripe-webhook-k8s
```

- **Pod not `Running` or not `1/1 Ready`** - inspect **`oc describe pod`** and logs (**`ImagePullBackOff`** → refresh **ECR** **`ecr-registry`** Secret; **`CreateContainerConfigError`** → missing **`stripe-webhook-secret`**; **`CrashLoopBackOff`** → logs).
- **`Endpoints` has no addresses** - often **readiness** never passes (app not listening, wrong probe path, or workload still starting). **`oc get pods -o wide`** and **`oc logs`** clarify.
- **Route targets wrong **`Service`** or port** - **`oc describe route …`** shows **To:** service and port; they must match **`k8s/service.yaml`** (**`go-stripe-webhook-k8s`**, port **8080**). **Path** on the Route should be **`/`** (or **`/livez`** must sit under whatever path prefix you configured).

**In-cluster check** (bypasses the public **Route**; expects **JSON** from the app):

```bash
oc run curl-probe --rm -it --restart=Never --image=curlimages/curl -- \
  curl -sS "http://go-stripe-webhook-k8s:8080/livez"
```

If that works but the **browser**/**`curl`** URL does not, focus on **Route** configuration; if it fails, focus on **Pods**/**Service**/**Endpoints**.

---

## 6. **“Red Hat OpenShift Service on AWS”** in the UI

Indicates **hosted OpenShift running on AWS** for that offering. It is **not** the same thing as **your learning AWS account** for **Terraform**/**ECR** unless **you** deploy **ROSA** in **your** org (Sandbox is **managed** separately). **ECR pulls** still use **your **`ecr-registry`** Secret**.

---

## 7. IaC snapshot (**optional **`git`)

When the Route matches what you want, export YAML for reproducibility (**strip **`status`** for a tidy commit**):

```bash
oc get route go-stripe-webhook-k8s -o yaml
```

Prefer storing **`openshift/route.yaml`** in **`git`** (**no Secrets** embedded).
