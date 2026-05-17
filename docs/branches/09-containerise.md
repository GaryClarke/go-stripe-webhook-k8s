# Branch 09: Containerise (`9-containerise`)

**Goal:** **Milestone 3** groundwork: **`Dockerfile`** (multi-stage **golang** builder to **`gcr.io/distroless/static-debian12:nonroot`**), **`.dockerignore`** for a small build context, and **`PLAN.md` / `README.md`** updates so **Done when** is explicit (local **`docker build` / `docker run`** plus a future **CI registry push**).

## What we added

- **`Dockerfile`**: static binary (**`CGO_ENABLED=0`**, **`-trimpath`**, **`-ldflags="-s -w"`**), **`TARGETOS` / `TARGETARCH`** (default **amd64**) for **BuildKit** / **`--platform`**, **`ENTRYPOINT ["/api"]`**, **`USER 65532`**, **`EXPOSE 8080`**.
- **`.dockerignore`**: excludes **`.git`**, **docs**, **`downstream/`**, **`terraform/`**, tests, **`.env`**, IDE noise, etc.; keeps **`cmd/`**, **`internal/`**, **`go.mod`**, **`go.sum`** for **`go build ./cmd/api`**.
- **`PLAN.md`**: Milestone 3 **Done when** split into **(1)** local container smoke **(2)** **GitHub Actions** build + push; note that **CI image job** may follow this commit.
- **`README.md`**: Docker commands, **`.dockerignore`**, Apple Silicon **`--platform`**, accurate CI wording; layout rows for **`Dockerfile`** / **`.dockerignore`**.
- **`go.mod`**: **`go 1.26.1`** (toolchain alignment).

## Files changed (high level)

- **`Dockerfile`**, **`.dockerignore`**, **`PLAN.md`**, **`README.md`**, **`go.mod`**

## How to verify

```bash
go test ./...
docker build -t go-stripe-webhook-k8s .
docker run --rm -p 8080:8080 -e STRIPE_WEBHOOK_SECRET=whsec_test go-stripe-webhook-k8s
curl -sS http://localhost:8080/livez
```

## Follow-ups

- **Milestone 3:** **`10-terraform-ecr-github-oidc`** (Terraform **OIDC** + **IAM** + **ECR**) - see **[10-terraform-ecr-github-oidc.md](10-terraform-ecr-github-oidc.md)**; then **`11-gh-actions-ecr-push`** (**GHA** image build + push). See **[PLAN.md](../../PLAN.md)**.
- **Milestone 4:** **`k8s/`** manifests and local cluster apply.
