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
| **Build** | `cmd/api` with `GET /livez`, `GET /readyz`, **`POST /webhooks/stripe`** (v1 stub is fine: accept POST, read body safely, **204** or **200**, log receipt; **no** Stripe signature verification until Milestone 2). |
| **Done when** | `go run ./cmd/api`; `curl` **GET** `/livez` and **POST** `/webhooks/stripe` behave as expected; `/readyz` as documented. |

**Suggested next branch:** e.g. **`4-webhook-stripe-stub`** (or similar) if probes and shutdown are already on **`main`**.

**Note on readiness:** For v1, `/readyz` may match `/livez` until **Milestone 7** adds shared dependencies (e.g. DB/Redis) for idempotency. Document the chosen rule under [Decisions](#decisions) when it changes.

---

## Milestone 2: Configuration and secrets

| | |
|--|--|
| **Learn** | Environment variables; ConfigMap vs Secret; Stripe webhook signing secret. |
| **Build** | Config for `STRIPE_WEBHOOK_SECRET`, `DOWNSTREAM_URL`, `PORT` (and any other required keys). |
| **Done when** | App reads config from the environment; **missing required config fails fast with a clear error**. |

**Logging:** Keep **simple** `log` lines for important paths (e.g. webhook received, config load failures). **Structured JSON** and **correlation IDs** are **Milestone 6**.

---

## Milestone 3: Containerise

| | |
|--|--|
| **Learn** | Dockerfile; multi-stage builds; **non-root user** and minimal image basics (prepares for **Pod securityContext** in Milestone 4). |
| **Build** | `Dockerfile`, `.dockerignore`. Image must run as **non-root** if you want **`runAsNonRoot: true`** on the Deployment without fighting the runtime. |
| **Done when** | `docker build` and `docker run` (with env file or `-e`) expose the API and health endpoints on the expected port. |

**Optional stretch:** If you aim for **`readOnlyRootFilesystem: true`** later, ensure the process needs **no writable layer** (or add a documented `emptyDir` mount when you hit that in Milestone 4).

---

## Milestone 4: First Kubernetes deploy

| | |
|--|--|
| **Learn** | Pod, Deployment, Service; probes; **Secrets** wiring; **resource requests/limits**; **Pod `securityContext`** (OpenShift-friendly); port alignment (`containerPort` ↔ `Service` ↔ `port-forward`). |
| **Build** | Core YAML under `k8s/` (see list below). |
| **Done when** | On a **local** Kubernetes cluster (Docker Desktop, Minikube, or kind): `kubectl apply -f k8s/`, pods ready, `kubectl port-forward svc/...` reaches webhook and health URLs; Secret-backed config is **documented** and **safe for git**. |

**Include in `k8s/` (or README-only where noted):**

- **`deployment.yaml`** and **`service.yaml`** as before.
- **Secrets:** **`k8s/secret.yaml`** with **placeholder** values *or* documented **`kubectl create secret generic ...`** / **`--dry-run=client -o yaml`** flow - **never commit real Stripe secrets**. Wire **`STRIPE_WEBHOOK_SECRET`** (and related keys) with **`envFrom`** or **`secretKeyRef`**.
- **`resources.requests` and `resources.limits`** (starter values are enough to learn the knobs).
- **`securityContext`:** e.g. **`runAsNonRoot: true`**, **`allowPrivilegeEscalation: false`**, and optionally **`readOnlyRootFilesystem: true`** when the image tolerates it (may need **`emptyDir`** for temp paths later).

This **is** a real deployment: workloads run on a real kubelet; the only deliberate scope choice is **Phase A** (local control plane). See [Deployment phases](#deployment-phases) before pushing the same image and manifests to a remote cluster.

Use a **single port convention** end-to-end (e.g. `8080`) to avoid confusion between Service `port`, `targetPort`, and the app `PORT`.

---

## Milestone 5: OpenShift routing

| | |
|--|--|
| **Learn** | OpenShift `Route` vs Kubernetes `Service`; TLS termination at the edge. |
| **Build** | `openshift/route.yaml` (or equivalent). |
| **Done when** | You can explain: Deployment runs Pods; Service exposes them **inside** the cluster; Route exposes the Service **outside** (with TLS as configured). |

For a **remote** OpenShift experience without standing up AWS yourself, the [Red Hat OpenShift Developer Sandbox](https://developers.redhat.com/developer-sandbox) (or your employer’s cluster) fits **Phase B**: build/push the image, apply manifests, add the Route.

---

## Milestone 6: Observability

| | |
|--|--|
| **Learn** | Structured logs; request or correlation IDs; operational log lines without secrets. |
| **Build** | Consistent JSON (or agreed format); log receipt, Stripe event id, outcome, errors. |
| **Done when** | `kubectl logs` is enough to trace a single webhook through the service. |

**Ordering:** This milestone sits **before idempotency** (Milestone 7) so logs help debug duplicate delivery and storage behaviour.

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

## Decisions

Record short, dated bullets as you go (examples below).

- *Example:* YYYY-MM-DD — `/readyz` equals process up until Redis is required.
- *Example:* YYYY-MM-DD — Standardise on port 8080 for app, Service targetPort, and examples.
- **2026-05-06** - **Milestone 6 = Observability**, **Milestone 7 = Idempotency** (swap so logs aid debugging idempotency work).

---

## Open questions

- None yet; add items here when a milestone surfaces a choice (e.g. Redis vs SQL for idempotency, managed Kafka vs self-run).
