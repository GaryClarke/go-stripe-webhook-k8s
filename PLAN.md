# Plan: Stripe webhook → Kubernetes service

This document is the **learning roadmap** and decision log. For day-to-day commands and layout, see [README.md](README.md).

## Purpose

Evolve a Stripe webhook integration from **serverless (Lambda)** to a **long-running HTTP service** packaged as a **container** and deployed on **Kubernetes** (with **OpenShift** routes where relevant). Preserve core domain logic while practising real operational concerns: config, probes, routing, idempotency, and observability.

## Architecture shift (summary)

```text
Before: Stripe → API Gateway → Lambda → downstream
After:  Stripe → HTTP API (Go) → processing / queue → downstream
                 └── running on Kubernetes (Pods, Service, Ingress/Route)
```

A later phase introduces **Kafka** to decouple ingestion from processing.

## Non-goals (v1)

- No Stripe Connect–specific flows unless explicitly added later.
- No storage of full payment card data or broad PCI scope expansion.
- No production-grade Helm chart or full GitOps story in early milestones.
- No bespoke retry or operations dashboard until basics are solid.
- **No AWS-centric deploy milestone in v1** (no EKS + IAM + VPC as the default learning path). You still deploy **Kubernetes** for real—first on a **local cluster**; optional **Phase B** can add a registry and a remote cluster (OpenShift sandbox, ROSA, EKS, etc.) without changing the app shape.

## Deployment phases

The **application** stays the same: Go HTTP server → Dockerfile → Deployment / Service / Route. Only **where the control plane runs** and **how the image reaches the nodes** changes.

| Phase | Where | What you add |
|--------|--------|----------------|
| **A — Local cluster (default through Milestone 4)** | Docker Desktop Kubernetes, Minikube, or kind | `kubectl apply`, same manifests; image via local daemon / `kind load` as needed |
| **B — Remote cluster (optional later)** | e.g. OpenShift Developer Sandbox, managed OpenShift/Kubernetes, or EKS | Container registry (`docker push`, pull secrets), `kubeconfig`, cluster Secrets; **same YAML** if you keep config portable |

**Why local first:** Learn Pods, Deployments, Services, probes, and logs without splitting attention across cloud networking and IAM. **Phase B** reuses the same artefacts and teaches registry + credentials when you want a public URL or team-aligned environment.

## How to use this plan

Each milestone lists **what to learn**, **what changes in the repo**, and **how you know you are done**. Implement **one milestone at a time**; prefer small commits and a working binary after each step.

**Branch discipline:** Use numbered git branches (e.g. `1-add-livez-and-readyz`) and add a short write-up under **[docs/branches/](docs/branches/README.md)** when useful. **Cursor / AI workflow** (chat shorthands, merge flow): single source of truth is **[cursor-rules.md](cursor-rules.md)**—do not duplicate those rules in this file.

---

## Milestone 1: HTTP service (local)

| | |
|--|--|
| **Learn** | HTTP handlers in Go; `/livez` and `/readyz`; graceful shutdown; **light operational logging** (e.g. one line when the webhook is hit - no structured JSON required yet). |
| **Build** | `cmd/api` with `GET /livez`, `GET /readyz`, **`POST /webhooks/stripe`** (v1 stub is fine: accept POST, read body safely, **204** or **200**, log receipt; **no** Stripe signature verification until Milestone 2). **`internal/dbg`** with **`DD`**: **`-tags debug`** selects **`spew`** to **stderr** and **`os.Exit(1)`** ( **`Recover`** -safe); default / release builds use a **no-op** **`DD`** so there is no dump or exit path unless you opt in. Local: **`go run -tags debug ./cmd/api`** or **`go test -tags debug ./...`**. Container / CI: build **without** **`debug`** (plain **`go build`** is enough). |
| **Done when** | `go run ./cmd/api`; `curl` **GET** `/livez` and **POST** `/webhooks/stripe` behave as expected; `/readyz` as documented. |

**Local-only scratch files:** If you add throwaway **`*.go`** snippets for experiments, use a dedicated name and list it in **your** global gitignore so it is never committed. This repo documents the convention as **`zzz_stripe_webhook_k8s_dd_scratch_only.go`** (unlikely to collide with normal source names; avoid a leading dot on the basename so the Go toolchain still compiles the file when you want it to).

**Milestone 1 status:** Complete on **`main`**: probes, webhook stub, graceful shutdown, **`Recover`**, **`internal/dbg`**, tests including **`TestAPI_Readyz`**. **Milestone 2** verification landed on **`8-stripe-webhook-verify`** (see [Milestone 2](#milestone-2-configuration-and-secrets)).

**Note on readiness:** For v1, `/readyz` may match `/livez` until **Milestone 7** adds shared dependencies (e.g. DB/Redis) for idempotency. Document the chosen rule under [Decisions](#decisions) when it changes.

**`cmd/api` layout:** HTTP routes are registered on **`(*App).routes()`** (`cmd/api/app.go`); handlers live in **`cmd/api/handlers.go`**. **`main`** loads config, **`NewApp`**, **`Recover(app.routes())`**, and server lifecycle only. **`POST /webhooks/stripe`** uses **`app.cfg.StripeWebhookSecret`** with **`stripe.ConstructEvent`**. **Future:** pass more dependencies on **`App`** (e.g. downstream clients).

**Milestone 1 carry-forward notes (non-blocking):**

- **`maxStripeWebhookBody`** ( **`http.MaxBytesReader`** cap) is fine as a **constant** in Milestone 1; a later milestone can load a limit from **config** if you want it tunable without a rebuild.
- **`Recover`** cannot reliably turn a response into **500** if the handler **already wrote headers**; **`cmd/api/recover.go`** documents that next to **`http.Error`**. Acceptable for Milestone 1; plan to refine when you add response-wrapping middleware in **Milestone 6** (see that milestone).

---

## Milestone 2: Configuration and secrets

| | |
|--|--|
| **Learn** | Environment variables; ConfigMap vs Secret; Stripe webhook signing secret. |
| **Build** | HTTP API config: **`PORT`** (default **8080**), required **`STRIPE_WEBHOOK_SECRET`**, webhook **signature verification** on the raw body; fail fast when **required** vars are missing. **`DOWNSTREAM_URL`** only when the app actually calls downstream (otherwise **optional** or **deferred** - see notes). Reuse or extend **`internal/config`** as needed (a Lambda-era **`Load()`** may already exist - evolve it for **`cmd/api`** rather than duplicating loaders). |
| **Done when** | App reads config from the environment; **missing required config fails fast with a clear error**; invalid Stripe signatures rejected with an appropriate HTTP status. |

**Milestone 2 status:** Complete on **`8-stripe-webhook-verify`**: required **`STRIPE_WEBHOOK_SECRET`** and **`PORT`** (default **8080**) from **`internal/config`**; **`stripe.ConstructEvent`** on the raw body and **`Stripe-Signature`** header; **400** on invalid payload or verification failure (aligned with Stripe's webhook examples); tests cover valid signed requests, missing signature, and invalid signature. Proceed to **Milestone 3** ([Containerise](#milestone-3-containerise)).

**Logging:** Keep **simple** `log` lines for important paths (e.g. webhook received, config load failures). **Structured JSON** and **correlation IDs** are **Milestone 6**.

**Stripe webhook verification:** Use **`stripe.ConstructEvent(body, stripeSignatureHeader, webhookSecret)`** from **`github.com/stripe/stripe-go/v85`** so verification runs on the **same raw bytes** read from **`r.Body`** (after **`http.MaxBytesReader`**).

**Git branch (suggested names):** Config work may use **`6-milestone-2-config`** or **`6-config-env`**; this repo completed verification on **`8-stripe-webhook-verify`**.

**Primary target:** load config from the environment and **fail fast** when **required** variables are missing.

**Suggested implementation order** (one branch or a small sequence):

1. **`internal/config`** (or extend existing package) for **`cmd/api`**.
2. **`PORT`** with default **8080**.
3. Require **`STRIPE_WEBHOOK_SECRET`** at startup (or wherever **`Load`** runs).
4. **`DOWNSTREAM_URL`**: **optional** until outbound HTTP exists; do **not** fail startup for a var that has **no behaviour** unless you deliberately want a future hook.
5. **`App`** struct (or equivalent) holding **`*config.Config`**; **`(*App).routes()`** / **`app.handleStripeWebhook`** (see Milestone 1 layout note).
6. Tests for config loading (missing var, default **`PORT`**, valid **`Load`**).
7. Stripe signature verification after config is wired.

**Roadmap context:** **Kubernetes / OpenShift** Secrets and **`securityContext`** stay in **Milestones 3–4** so they do not block local config work. **Observability** before **idempotency** (**Milestones 6 then 7**) remains the agreed order.

---

## Milestone 3: Containerise

| | |
|--|--|
| **Learn** | Dockerfile; multi-stage builds; **non-root user** and minimal image basics (prepares for **Pod securityContext** in Milestone 4). **AWS ECR** as the container registry. **GitHub Actions OIDC** (OpenID Connect) so CI can **assume an IAM role** **without** long-lived AWS access keys in GitHub. Minimal **Terraform** for **OIDC provider**, **IAM role**, and **ECR repository** (this repo stays **IaC-first**; **Kubernetes** is **not** ECS). |
| **Build** | **`Dockerfile`**, **`.dockerignore`**. Image must run as **non-root** for **`runAsNonRoot: true`** on the Deployment later. **Registry + IAM (branch `10-terraform-ecr-github-oidc`):** remove **legacy** **`terraform/`** (parent **Lambda + API Gateway** stack; continued in the **original** repo if still needed). Add a **new** Terraform root (e.g. **`infra/terraform/`** or agreed path) with its **own** remote state **key** (do **not** reuse Lambda state). Define **GitHub OIDC** identity provider (once per learning account), an **IAM role** trusted by **this** GitHub repo/ref for **ECR push**, and an **ECR repository** for this service. **CI image job (branch `11-gh-actions-ecr-push`):** extend **`.github/workflows`** so pushes to **`main`** run **`docker buildx`** against this **`Dockerfile`**, authenticate via **`aws-actions/configure-aws-credentials`** (**OIDC** → role from **`10`**), **login** to **ECR**, and **push** a tagged image (e.g. **`${{ github.sha }}`**). Keep **Go** jobs (**`go build`**, **`go test`**, lint) as gates. Document **role ARN** / registry URL via **Terraform outputs** and **README** (no secrets in git). |
| **Done when** | **(1)** **`docker build`** and **`docker run`** (with **`.env`**, **`--env-file`**, or **`-e STRIPE_WEBHOOK_SECRET=...`**) expose the API and health endpoints on the expected port (unsigned webhook **POST**s **400**; see Milestone 2). **(2)** Terraform from **`10-terraform-ecr-github-oidc`** is **applied** in the learning account: **ECR repo** + **OIDC** + **CI role** exist. **(3)** A workflow run on **`main`** from **`11-gh-actions-ecr-push`** **builds** and **pushes** a tagged image to **that** **ECR**; **Go** jobs remain green. |

**Git branch sequencing (Milestone 3):**

| Branch (git) | Scope |
|--------------|--------|
| **`10-terraform-ecr-github-oidc`** | Drop legacy **`terraform/`** from **this** fork. Add Terraform for **GitHub OIDC** + **IAM** (CI **AssumeRoleWithWebIdentity**) + **ECR repository**; document **`terraform output`** and apply steps. |
| **`11-gh-actions-ecr-push`** | **GitHub Actions** job: **OIDC** to AWS, **build**, **push** to **ECR**, tagging convention. |

**Out of scope for this milestone (stretch later):** multi-environment promotion pipelines (dev → staging → prod), deploy steps per env, and post-deploy smoke beyond what you already run in CI. **Milestone 4** can still load the image locally (**`kind load`**, etc.); a registry makes **Phase B** (remote cluster pulls the same image) straightforward.

**Optional stretch:** If you aim for **`readOnlyRootFilesystem: true`** later, ensure the process needs **no writable layer** (or add a documented `emptyDir` mount when you hit that in Milestone 4).

**Later / clarification (no action required for Milestone 3):** After **`aws ecr get-login-password | docker login ...`** to **ECR**, the Docker CLI sometimes prints a generic warning about using a **personal access token** (**Docker Hub**). That message is **not** about **AWS ECR**. **ECR** uses a **short-lived** registry password from **AWS**; access control is **IAM** (who may call **GetAuthorizationToken** / push), not Docker Hub PATs. Revisit only if we want a one-liner in **README** for newcomers.

**Milestone 3 status:** **(1)** done on **`main`** (**`9-containerise`**). **(2)** done on **`main`** (**`10-terraform-ecr-github-oidc`**, Terraform applied in the learning account). **(3)** **`11-gh-actions-ecr-push`** adds the **ECR** push job - treat **Milestone 3** as fully satisfied once **`11`** is on **`main`**, **Actions** is green, and **ECR** shows the new image tag(s).

---

## Milestone 4: First Kubernetes deploy

**Background first (lesson):** Before writing YAML, walk through a **full orientation**: what **Kubernetes** is (and how it differs from **ECS** and from **EKS**); **control plane** vs **nodes** / **kubelet**; **Pods**, **ReplicaSet/Deployment**, **Service**, **Namespace**; **declarative manifests** and **`kubectl apply`**; **container image** + **`imagePullPolicy`** vs **local** **`kind load`**; **probes** and why they matter for your **`/livez`** / **`/readyz`**; **Secrets** vs **ConfigMaps**. That can be an in-chat deep dive, a short doc under **`docs/`** (add a file when you open this milestone), or both. **Then** implement the **`k8s/`** list below so each object ties back to that mental model.

| | |
|--|--|
| **Learn** | Same as the orientation above, then in practice: Pod, Deployment, Service; probes; **Secrets** wiring; **resource requests/limits**; **Pod `securityContext`** (OpenShift-friendly); port alignment (`containerPort` ↔ `Service` ↔ `port-forward`). |
| **Build** | Core YAML under `k8s/` (see list below). |
| **Done when** | On a **local** Kubernetes cluster (Docker Desktop, Minikube, or kind): `kubectl apply -f k8s/`, pods ready, `kubectl port-forward svc/...` reaches webhook and health URLs; Secret-backed config is **documented** and **safe for git**. |

**Include in `k8s/` (or README-only where noted):**

- **`deployment.yaml`** and **`service.yaml`** as before.
- **Secrets:** **`k8s/secret.yaml`** with **placeholder** values *or* documented **`kubectl create secret generic ...`** / **`--dry-run=client -o yaml`** flow - **never commit real Stripe secrets**. Wire **`STRIPE_WEBHOOK_SECRET`** (and related keys) with **`envFrom`** or **`secretKeyRef`**.
- **`resources.requests` and `resources.limits`** (starter values are enough to learn the knobs).
- **`securityContext`:** e.g. **`runAsNonRoot: true`**, **`allowPrivilegeEscalation: false`**, and optionally **`readOnlyRootFilesystem: true`** when the image tolerates it (may need **`emptyDir`** for temp paths later).

This **is** a real deployment: workloads run on a real kubelet; the only deliberate scope choice is **Phase A** (local control plane). See [Deployment phases](#deployment-phases) before pushing the same image and manifests to a remote cluster.

Use a **single port convention** end-to-end (e.g. `8080`) to avoid confusion between Service `port`, `targetPort`, and the app `PORT`.

**Load balancing (two layers):** A **Service** gives **stable cluster-internal** DNS/IP and sends traffic to **Ready** Pods that match the **selector**; with **`replicas > 1`**, that behaves like **in-cluster** distribution across Pods. **Milestone 4** can stay at **`replicas: 1`**; raising replicas (e.g. to **3**) is a useful **optional stretch** to see multiple Pods behind one Service. **Edge** load balancing (clients → cloud load balancer → **Ingress** or **OpenShift Route** → **Service** → Pods) is **Milestone 5** / **Phase B**, not required for local **ClusterIP** + **`kubectl port-forward`**.

**Milestone 4 status:** **`12-k8s-first-deploy`** adds **`k8s/deployment.yaml`** + **`k8s/service.yaml`**; **`kubectl`-created Secrets** (**`stripe-webhook-secret`**, **`ecr-registry`**) documented in **`docs/branches/12-k8s-first-deploy.md`** (never commit real **`whsec_*`** values). Treat **Done when** as satisfied on **`main`** once you can **`kubectl apply -f k8s/`**, see **`READY`**, **`kubectl port-forward svc/go-stripe-webhook-k8s 8080:8080`**, and **`curl`** **`/livez`** / **`/readyz`**.

---

## Milestone 5: OpenShift routing

| | |
|--|--|
| **Learn** | OpenShift `Route` vs Kubernetes `Service`; TLS termination at the edge. |
| **Build** | `openshift/route.yaml` (or equivalent). |
| **Done when** | You can explain: Deployment runs Pods; Service exposes them **inside** the cluster; Route exposes the Service **outside** (with TLS as configured). |

For a **remote** OpenShift experience without standing up AWS yourself, the [Red Hat OpenShift Developer Sandbox](https://developers.redhat.com/developer-sandbox) (or your employer’s cluster) fits **Phase B**: build/push the image, apply manifests, add the Route.

**Milestone 5 status:** **`13-openshift-route`** on **`main`** adds **`openshift/route.yaml`** (portable **`Route`**, **`tls.termination: edge`**, insecure redirect to **`https`**, **`targetPort: http`** to **`go-stripe-webhook-k8s`** Service) and **`docs/openshift/`**. **Done when** on **`Phase B`**: **`oc apply -f k8s/`**, **`oc apply -f openshift/route.yaml`**, public **`curl`** **`/livez`** / **`/readyz`** return **`cmd/api`** JSON, and you can narrate **Pods** → **ClusterIP Service** → **Route** (router terminates **TLS**). **Automated **`oc apply`** from CI + Secret lifecycle** stays the **Stretch** section below.

---

## Milestone 6: Observability

| | |
|--|--|
| **Learn** | Structured logs; request or correlation IDs; operational log lines without secrets; **HTTP middleware** for request-scoped logging (stdout → later Datadog/ELK/CloudWatch via the platform). |
| **Build** | Consistent JSON (or agreed format) to **stdout**; **response-wrapping** middleware (request ID, request start/end or equivalent); log Stripe **event id**, **type**, outcome, and errors (no secrets or raw card data). Revisit **`Recover`** in **`cmd/api/recover.go`** with that middleware so panics after the response has started do not fight **`http.Error`** (track response-started, or log-only on panic). Optional **`docs/branches/`** note: example **`oc logs`** / **`grep`** today and example field searches for a future central system. |
| **Done when** | `kubectl logs` / `oc logs` is enough to trace a single webhook through the service (answer: what happened to this event?). |

**Ordering:** This milestone sits **before idempotency** (Milestone 7) so logs help debug duplicate delivery and storage behaviour.

**Out of scope for M6:** Installing Datadog, ELK, CloudWatch, or cluster logging stacks; metrics, tracing, dashboards, alerting.

**Recover / partial response (deferred from Milestone 1, in scope for M6):** If a handler **panics after** it has already started the response (e.g. **`WriteHeader`** or body bytes on the wire), **`http.Error`** in **`Recover`** cannot reliably turn the client-visible outcome into **500** - see **`cmd/api/recover.go`**. **M6** includes aligning **middleware + `Recover`** (e.g. wrapped **`http.ResponseWriter`**, **`http.ResponseController`**, or log-only after write started) so panic handling does not double-write or corrupt the stream.

**Milestone 6 status:** **Complete** on **`main`** via **`14-structured-logging`**. Branch doc: **[docs/branches/14-structured-logging.md](docs/branches/14-structured-logging.md)**. Shipped: JSON **`slog`**, **`RequestLog`**, **`request_id`** on context, **`logmsg`** contract, handler correlation. **Phase 4** (**Recover** + **`response_started`** on panic after write started) deferred to a follow-up branch. **Sandbox:** after CI pushes ECR **`latest`**, **`oc rollout restart`** and confirm **`oc logs`** JSON trace (local verify done). **Next:** **Milestone 7** (idempotency).

**Implementation order (summary):**

1. **Phase 0** - Learn: stdout pipeline, read **`main`**, **`recover.go`**, **`handlers.go`**; lock request ID + JSON field contract (see branch doc).
2. **Phase 1** - **`log/slog`** JSON to stdout.
3. **Phase 2** - Response-wrapping + request middleware; **`Recover(RequestLog(routes))`** in **`main`**.
4. **Phase 3** - Stripe handler structured **`msg`** lines; no secrets in logs.
5. **Phase 4** - **Recover** vs response-started; tests match production stack.
6. **Phase 5** - Verify **`go test`**, local run, **`oc logs`**; document example **`grep`** / future ELK/Datadog-style queries in branch doc.

---

## Milestone 7: Idempotency

| | |
|--|--|
| **Learn** | Stripe retries; duplicate delivery; `event.id`; **shared state across replicas**. |
| **Build** | Persist processed event IDs (e.g. DB or Redis)—not in-memory dedupe for real behaviour. |
| **Done when** | Sending the same Stripe event twice results in **one** effective processing path. |

In-memory dedupe is acceptable only as a **demonstration** with explicit “single replica only” caveats.

---

## Milestone 8: Kafka

| | |
|--|--|
| **Learn** | Topics; producer and consumer; offsets; **at-least-once** delivery and idempotent consumers. |
| **Build** | Webhook publishes to Kafka; separate consumer consumes and persists or displays data. |
| **Done when** | `Stripe webhook → Kafka → consumer` works in a **local** dev setup (e.g. Compose or lightweight broker), with README steps documented. |

You do not need a full cluster-distributed Kafka on day one; pick one local story and document it.

---

## Stretch (not numbered): OpenShift deploy job + secret automation

**Why:** **Milestones 4–5** use **documented **`kubectl` / `oc`** commands** to create **pull** and **Stripe** **Secrets**. That is deliberate for learning. **Employer-style production** usually **automates** how **Secret** *values* arrive (no long-lived human **`oc create`** for every env). This stretch is **explicitly out of scope** for the numbered **M6–M8** ordering so **Observability → Idempotency → Kafka** stays intact.

| | |
|--|--|
| **Learn** | **Pipeline deploy** (**`oc login`** / **`kubectl`** from **GitHub Actions**, **Tekton**, etc.) using **short-lived** CI credentials; **values** from **CI secret store** (**GitHub Secrets**, **Vault**) into cluster **Secrets** at deploy time. **Or** **operator-driven** sync (**External Secrets Operator**, **Secrets Store CSI**) from **AWS Secrets Manager**, **Azure Key Vault**, **HashiCorp Vault**, etc. **Or** **encrypted Git** (**Sealed Secrets**, **SOPS**). **ECR** pulls: **rotating** **`docker-registry`** **Secret** (**CronJob** / pipeline) or **platform-managed** pull (**cluster** pull secret, **mirror**, **ROSA**/cloud integration). |
| **Build** | Pick **one** pattern and document it (**README** + **`docs/branches/`**): e.g. **GHA** job **`oc apply`** + **`oc create secret …`** from **`GH`** secrets (no values in logs), or **ExternalSecret** manifest in **git** pointing at a **store path** per environment. |
| **Done when** | A **sandbox or non-prod** namespace can be **reprovisioned** without you **manually** pasting **`whsec_`** or **ECR passwords** on your laptop (one **pipeline** or **sync** run creates/updates **Secrets**). |

**Does production “work” without this stretch?** Yes, for a **narrow** definition: **Sandbox** or a **small** env can run with **runbook** **`oc` steps** and **git** manifests. **Real** org **prod** almost always adds **automation** above for **rotation**, **audit**, and **least privilege** — that is what this stretch tracks.

---

## Decisions

Record short, dated bullets as you go (examples below).

- *Example:* YYYY-MM-DD — `/readyz` equals process up until Redis is required.
- *Example:* YYYY-MM-DD — Standardise on port 8080 for app, Service targetPort, and examples.
- **2026-05-06** - **Milestone 6 = Observability**, **Milestone 7 = Idempotency** (swap so logs aid debugging idempotency work).
- **2026-05-06** - HTTP routes live in **`cmd/api` `(*App).routes()`** and **`handlers.go`**; **`App`** holds **`*config.Config`** (see Milestone 1 layout note).
- **2026-05-07** - **`internal/dbg.DD`**: **`//go:build debug`** implementation (**`spew`** + **`os.Exit(1)`**); **`//go:build !debug`** no-op for release. Optional scratch filename **`zzz_stripe_webhook_k8s_dd_scratch_only.go`** is for personal global gitignore only, not a committed artefact.
- **2026-05-08** - **Milestone 1** marked **complete** on **`main`**; **Milestone 2** **approved** shape: **`6-milestone-2-config`** or **`6-config-env`**, **`Load`**, fail-fast **`STRIPE_WEBHOOK_SECRET`**, **`PORT`** default **8080**, **`application` struct + config**, tests, then **Stripe** signature verification; **`DOWNSTREAM_URL`** **optional**/deferred until used. **Observability** (**Milestone 6**) before **idempotency** (**Milestone 7**); cluster **Secrets** / **`securityContext`** stay **Milestones 3–4** (no change).
- **2026-05-08** - **`8-stripe-webhook-verify`**: **`stripe.ConstructEvent`** for **`POST /webhooks/stripe`**; **400** on missing / invalid **`Stripe-Signature`** or bad payload; tests use **`stripe.GenerateTestSignedPayload`**.
- **2026-05-13** - **Milestone 3** includes **GitHub Actions** **build + push** of the container image to an **OCI registry** (cohesive with **`Dockerfile`** / **`.dockerignore`**); **multi-env deploy** stays **stretch** / later.
- **2026-05-21** - **Stretch** (not a new milestone number): document **OpenShift deploy job + secret automation** in **`PLAN`** so **M4–M5** stay **imperative **`oc`/`kubectl` secrets** for teaching, while **pipeline / operator / sealed** patterns are an explicit path toward **employer-style prod** without delaying **M6–M8**.
- **2026-05-27** - **Milestone 6 request ID:** honor inbound **`X-Request-Id`** when present and valid, else generate **UUID v4**; store on **`context`**; log **`request_id`** on every structured line. **`event_id`** (Stripe) stays separate for webhook tracing and later idempotency.
- **2026-05-27** - **Milestone 6 logging:** **`log/slog`** with **`JSONHandler`** to **stdout**; migrate **`cmd/api`** off **`log`** / **`log.Printf`**. **`App`** holds base **`*slog.Logger`**; request middleware attaches a request-scoped logger (with **`request_id`**) on **`context`** for handlers.
- **2026-05-27** - **Milestone 6 access logs:** **`RequestLog`** middleware skips **`request_started`** / **`request_completed`** for **`GET /livez`** and **`GET /readyz`** (probe noise); all other routes logged at **info**. Handlers may still use minimal probe logs only if needed (default: none).
- **2026-05-27** - **Milestone 6 Stripe tracing:** **`request_id`** on every HTTP log line (middleware + handlers). After successful **`ConstructEvent`**, add **`event_id`** and **`event_type`** on all webhook handler logs for that request; on verify/read failures, log **`request_id`** only (no **`event_id`** until parsed). **`event_id`** is the stable key across retries and future milestones (idempotency, Kafka); **`request_id`** is one delivery attempt.
- **2026-05-27** - **Milestone 6 handler stack:** **`Recover(RequestLog(app.routes()))`** in **`main`** - **Recover** outermost (catches panics in logging middleware and handlers); **`RequestLog`** wraps routes and the response-recording **`ResponseWriter`**.

## Open questions

- **Secret / deploy automation:** after **Milestone 5** (Route + public URL), pick one **Stretch** approach (e.g. **GitHub Actions **`oc apply`** + secret sync from **`GH` Secrets**, or **External Secrets Operator** sketch) or defer if **Barclays** defines a standard pattern.
- *Examples for later:* Redis vs SQL for idempotency (Milestone **7**); managed Kafka vs self-run (Milestone **8**).
