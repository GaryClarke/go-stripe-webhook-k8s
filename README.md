# go-stripe-webhook-k8s

A **Kubernetes-oriented** Stripe webhook service in Go. This repository continues the domain and parsing logic from an earlier **AWS Lambda** integration engine, but the deployment target is **containers on Kubernetes or OpenShift**, not API Gateway + Lambda.

For the learning roadmap, milestones, and design notes, see **[PLAN.md](PLAN.md)**.

## What you get (target shape)

- **HTTP API**: `POST /webhooks/stripe` (validate signature, process or hand off events, respond quickly).
- **Operations**: `GET /livez` and `GET /readyz` for probes.
- **Runtimes**: local process, Docker image, then manifests for Kubernetes / Route for OpenShift.

Legacy Lambda and Terraform artefacts may remain in the tree during the transition; new work follows [PLAN.md](PLAN.md).

## Requirements

- [Go](https://go.dev/dl/) (see `go.mod` for the toolchain version).
- Optional: Docker, `kubectl`, and a **local** Kubernetes cluster (Docker Desktop Kubernetes, Minikube, or kind) when you reach Milestones 3–4. Remote cluster and registry are optional later; see [PLAN.md — Deployment phases](PLAN.md#deployment-phases).

## Local development

### Milestone 1 (HTTP server)

When `cmd/api` exists, the intended flow is:

```bash
go mod tidy
cp .env.example .env   # edit .env: STRIPE_WEBHOOK_SECRET is required for cmd/api
go run ./cmd/api
```

Optional **debug dumps** (**`internal/dbg.DD`**, **`spew`** + exit): **`go run -tags debug ./cmd/api`** or **`go test -tags debug ./...`** (see [PLAN.md](PLAN.md) Milestone 1).

Verify liveness:

```bash
curl -sS http://localhost:8080/livez
```

Use the port from your config (default `8080` once wired). See [PLAN.md](PLAN.md) Milestone 1 for the full “done when” checklist.

Historical Lambda / queue notes from the parent project live in [docs/PROJECT_KNOWLEDGE.md](docs/PROJECT_KNOWLEDGE.md).

## Tests and quality

```bash
go test ./...
make lint
```

## Docker (Milestone 3)

From the repo root (see **`.dockerignore`** for what enters the build context):

```bash
docker build -t go-stripe-webhook-k8s .
docker run --rm -p 8080:8080 --env-file .env go-stripe-webhook-k8s
# or without a file, for a quick check:
# docker run --rm -p 8080:8080 -e STRIPE_WEBHOOK_SECRET=whsec_test go-stripe-webhook-k8s
```

Adjust image name, port, and env file path to match your setup. On **Apple Silicon**, use **`docker build --platform linux/arm64 ...`** if you want a native **arm64** image (the **`Dockerfile`** uses **`TARGETARCH`**).

**CI:** **`.github/workflows/ci.yaml`** on **`main`**: **`go build`**, **`go test`**, **`make lint`**, then **`docker buildx`** **build and push** to **ECR** via **OIDC** ( **Actions Variables** e.g. **`AWS_ROLE_ARN`** from Terraform output - see **[docs/branches/11-gh-actions-ecr-push.md](docs/branches/11-gh-actions-ecr-push.md)** ). No AWS access keys in git. Multi-environment deploy stays optional stretch.

## Kubernetes (Milestones 4–5)

**Local first:** Run a real cluster on your machine (Docker Desktop Kubernetes, Minikube, or kind), then `kubectl apply -f k8s/`. That is a genuine deploy: same **Deployment** and **Service** pattern you will use elsewhere; you are only avoiding cloud IAM/VPC complexity until in-cluster concepts are familiar.

- Apply manifests under `k8s/` (and OpenShift `Route` under `openshift/` when present).
- Use `kubectl port-forward` to reach the Service from your machine until an Ingress or Route is configured.

**Later (optional):** Target a **remote** cluster (**Phase B**, e.g. OpenShift Developer Sandbox) that **pulls** the image your CI publishes (**Milestone 3**). Same **Dockerfile** and manifests; add registry credentials and **kubeconfig** as needed. See [PLAN.md — Deployment phases](PLAN.md#deployment-phases).

Details and ordering: [PLAN.md](PLAN.md).

## Layout (evolving)

| Path | Role |
|------|------|
| `Dockerfile` | Multi-stage image: **`cmd/api`** binary on **distroless**, non-root. |
| `.dockerignore` | Keeps Docker build context small; see [Milestone 3](PLAN.md#milestone-3-containerise). |
| `infra/terraform/` | IaC for **ECR** + **GitHub OIDC** IAM for CI image push; run Terraform only from this directory. Outputs (role ARN, registry URL) after **`apply`**: see **[docs/branches/10-terraform-ecr-github-oidc.md](docs/branches/10-terraform-ecr-github-oidc.md)**. |
| `cmd/api` | HTTP service entrypoint for local and container runs. |
| `internal/` | Shared packages (config, engine, dbg, etc.). |
| `testdata/` | Stripe webhook fixtures. |
| `k8s/` | Kubernetes manifests (as milestones progress). |
| `openshift/` | OpenShift `Route` and related objects (Milestone 5). |
| `docs/` | Deeper design history from the Lambda era. |

## Contributing / learning flow

Implement **one milestone at a time**, keep `README.md` usage accurate, and record decisions in [PLAN.md](PLAN.md). Numbered git branches and optional notes: [docs/branches/README.md](docs/branches/README.md) (Lambda-era logs: [docs/original-branches/README.md](docs/original-branches/README.md)). **Cursor / AI conventions:** [cursor-rules.md](cursor-rules.md).
