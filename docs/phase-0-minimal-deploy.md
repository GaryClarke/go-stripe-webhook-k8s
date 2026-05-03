# Phase 0: Minimal deployment (healthz)

Get a single Go Lambda behind API Gateway returning 200 for `GET /healthz`. No app logic yet. Assumes no prior AWS deployment experience.

---

## What we're building

- **Public URL:** e.g. `https://abc123.execute-api.eu-west-1.amazonaws.com/healthz`
- **Behaviour:** `GET /healthz` returns HTTP 200 and a small JSON body (e.g. `{"status":"ok"}`).
- **Under the hood:** API Gateway receives the request, invokes a Lambda function, Lambda runs our Go binary and returns the response, API Gateway sends it back to the client.

---

## Prerequisites (on your machine)

Before any Terraform or deployment:

| Item | What it is | How to get it |
|------|------------|----------------|
| **AWS account** | An account at aws.amazon.com. Needed to create resources. | Sign up at aws.amazon.com (free tier is enough). |
| **AWS CLI** | Command-line tool that talks to AWS using your identity. | Install: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html |
| **AWS credentials** | Access key + secret so the CLI (and Terraform) can act as you. | In AWS Console: IAM → Users → your user → Security credentials → Create access key. Then run `aws configure` and enter Access Key ID and Secret. |
| **Terraform** | Tool that creates/updates AWS resources from `.tf` files. | Install: https://developer.hashicorp.com/terraform/install |
| **Go** | To build the Lambda binary. | You already have this. |

Verification:

- `aws sts get-caller-identity` prints your account and user (credentials work).
- `terraform -version` runs.
- `go version` runs.

---

## AWS concepts we'll use (minimal)

- **Lambda:** Runs a single function (our Go code) when invoked. No server to manage. We give AWS a zip of our binary; it runs it per request.
- **API Gateway (HTTP API):** The public HTTP endpoint. We define a route (e.g. `GET /healthz`) and say "when someone hits this, call this Lambda."
- **IAM:** Who can do what. We need: (1) A role the Lambda runs as. (2) Permission for API Gateway to invoke that Lambda. Terraform will create these.

---

## AWS resources we'll create (and why)

| Resource | Purpose |
|----------|---------|
| **Lambda function** | Runs our Go binary. One invocation per request. |
| **Lambda execution role** | IAM role the Lambda runs under (e.g. permission to write logs). |
| **API Gateway HTTP API** | The HTTP endpoint (the URL people hit). |
| **API Gateway route** | `GET /healthz` mapped to our Lambda. |
| **API Gateway integration** | "Call this Lambda when the route is hit." |
| **Lambda permission** | Allows API Gateway to invoke the Lambda (without this, API Gateway gets 403). |
| **S3 bucket** (state) | Stores Terraform state so CI and other machines can run `terraform plan` against the real state. Created once, outside main Terraform or via bootstrap. |
| **DynamoDB table** (optional) | State locking: prevents two `terraform apply` runs from corrupting state. |

We create the app resources with Terraform. The S3 bucket and DynamoDB table for state are created first (manually or via a small bootstrap Terraform), then we add the backend config.

---

## Files and artifacts (checklist)

### In the repo (we create these)

| Path | Purpose |
|------|---------|
| `terraform/` | Directory for all Terraform files. |
| `terraform/main.tf` | Terraform block, provider, IAM, Lambda, API Gateway, route, integration, permission, output. All in one file. |
| `cmd/ingest/main.go` | Go Lambda: receives API Gateway event, returns 200 + JSON. (healthz is a route, not a separate cmd.) |
| `go.mod` (repo root) | Go module; add dependency `github.com/aws/aws-lambda-go` for Lambda. |
| `Makefile` | Target `build-ingest`: builds for Linux, zips to `build/bootstrap.zip`. |
| `.github/workflows/ci.yml` | CI: build, test, lint, terraform validate/fmt/plan. |
| `docs/phase-0-minimal-deploy.md` | This document. |

### Generated / local (not committed)

| Artifact | Purpose |
|----------|---------|
| `terraform/.terraform/` | Terraform plugins (after `terraform init`). |
| `terraform/.terraform.lock.hcl` | Lock file for provider versions (can be committed). |
| `terraform/terraform.tfstate` | With S3 backend, state lives in S3. Local file no longer used after migration. |
| `terraform/lambda.zip` or `build/bootstrap.zip` | Zip containing the Go binary named `bootstrap`. Lambda expects the handler to be named `bootstrap` for custom runtime / Go. |
| Lambda deployment package | Same zip, uploaded to Lambda via Terraform. |

### Naming we use

- Lambda: `integration-engine-payments-events-ingest`
- API: `integration-engine`
- Route: `GET /healthz`

---

## Steps in order

Do these in sequence. Each step can be one "chip" of work.

### 1. AWS account and CLI

- [ ] Create an AWS account (if needed).
- [ ] Install AWS CLI.
- [ ] Create an IAM user (or use root once for learning only), create an access key.
- [ ] Run `aws configure`, enter Access Key ID, Secret Access Key, and default region (e.g. `eu-west-1`).
- [ ] Run `aws sts get-caller-identity` and confirm it prints your account and user.

### 2. Terraform directory and provider

- [ ] Create `terraform/` in the repo.
- [ ] Add `main.tf`: `terraform` block (required version, `required_providers` with `aws` ~> 5.0), `provider "aws"` (region = `eu-west-1` or your choice).
- [ ] Run `terraform init` in `terraform/` (downloads AWS provider).

### 3. Lambda execution role (IAM)

- [ ] In Terraform: create an IAM role that Lambda can assume (`aws lambda.amazonaws.com` as principal).
- [ ] Attach the managed policy `AWSLambdaBasicExecutionRole` so the function can write to CloudWatch Logs.
- [ ] No permissions for SQS, DynamoDB, etc. yet; we add those when we need them.

### 4. Lambda function (placeholder)

- [ ] In Terraform: resource `aws_lambda_function` with: runtime (e.g. `provided.al2023` for custom runtime, or use Go 1.x if you prefer), handler `bootstrap`, a placeholder zip (e.g. a tiny zip you create once), role = the role from step 3.
- [ ] For custom runtime, the zip must contain an executable named `bootstrap`; Lambda runs `./bootstrap`.
- [ ] Run `terraform apply` and confirm the function is created (you can invoke it from console or CLI to see it fail until we add real code).

### 5. Go Lambda code (ingest handler)

- [ ] Add `github.com/aws/aws-lambda-go` to the Go module.
- [ ] Create `cmd/ingest/main.go`: a handler that takes `events.APIGatewayV2HTTPRequest` and returns `events.APIGatewayV2HTTPResponse` with status 200 and body `{"status":"ok"}`. Call `lambda.Start(handler)` in `main`.
- [ ] Build for Linux: `make build-ingest` (or `GOOS=linux GOARCH=amd64 go build -o build/bootstrap ./cmd/ingest`, then zip to `build/bootstrap.zip`).
- [ ] Update Terraform to point the Lambda at this zip: `filename = "${path.module}/../build/bootstrap.zip"`.
- [ ] Run `terraform apply`. Optionally test the function in the AWS Console (Test tab) with a sample API Gateway event.

### 6. API Gateway HTTP API

- [x] In Terraform: create `aws_apigatewayv2_api` with `protocol_type = "HTTP"`. This gives you an API (no URL yet until we add a stage).
- [x] Create `aws_apigatewayv2_stage` (e.g. `default`) so the API has an invoke URL.

### 7. Connect API Gateway to Lambda

- [x] Create `aws_apigatewayv2_integration`: type `AWS_PROXY`, `integration_uri` = Lambda invoke ARN, `payload_format_version = "2.0"`.
- [x] Create `aws_apigatewayv2_route`: `route_key = "GET /healthz"`, `target` = the integration ID.
- [x] Grant API Gateway permission to invoke the Lambda: `aws_lambda_permission` with `action = "lambda:InvokeFunction"`, `principal = "apigateway.amazonaws.com"`, and the API Gateway source ARN (so only our API can invoke it).

### 8. Build and deploy

- [x] Add a Makefile target `build-ingest` that: builds the Go binary for Linux, zips to `build/bootstrap.zip`.
- [x] Terraform references the zip at `../build/bootstrap.zip`. Run `terraform apply` after every change to the zip.
- [x] Output `api_url` in Terraform for the base invoke URL. Full healthz URL: `<api_url>/healthz`.

### 9. Verify

- [x] From your machine: `curl https://<api-url>/healthz`. Expect 200 and `{"status":"ok"}` (or whatever body you return).
- [x] Check Lambda logs in CloudWatch (Log group: `/aws/lambda/<function-name>`).

### 10. Document and tidy

- [x] Add `terraform/` to `.gitignore` for `.terraform/`, `*.tfstate`, `*.tfstate.*`, `*.zip` if you don't want to commit state or zips. Optionally commit `.terraform.lock.hcl`.
- [x] Update `docs/PROJECT_KNOWLEDGE.md` or branch doc to note that Phase 0 is "minimal healthz deploy" and list the steps/file locations.
- [ ] Optional: add a short `terraform/README.md` with "how to apply" and "how to get the URL".

### 11. S3 backend for Terraform state

- [ ] Create an S3 bucket for state (e.g. `integration-engine-terraform-state`) in your region. Enable versioning (recommended for recovery).
- [ ] Optionally create a DynamoDB table (e.g. `terraform-locks`) for state locking. Use `LockID` as partition key (string).
- [ ] Add a `backend "s3"` block to the `terraform` block in `main.tf` (or use `-backend-config` so bucket/table names are not hardcoded). Example:

  ```hcl
  backend "s3" {
    bucket         = "integration-engine-terraform-state"
    key            = "integration-engine/terraform.tfstate"
    region         = "eu-west-1"
    dynamodb_table = "terraform-locks"
  }
  ```

### 12. Migrate state to S3

- [ ] Run `cd terraform && terraform init`. Terraform will detect the new backend and ask: "Copy existing state to the new backend?" Answer **yes**.
- [ ] Verify: run `terraform plan`. It should show "No changes" (your resources already exist).
- [ ] You can remove the local `terraform.tfstate` and `terraform.tfstate.backup`; state now lives in S3.

### 13. GitHub Actions CI

- [ ] Create `.github/workflows/ci.yml` (or similar). Jobs: build (go build), test (go test), lint (make lint), terraform (validate, fmt -check, init, plan).
- [ ] Configure AWS credentials in GitHub: either OIDC (recommended) or repository secrets `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`.
- [ ] Push and confirm the workflow runs. `terraform plan` in CI validates config against the real S3 state.

---

## One-page file checklist

**Repo files (what we use):**

- `terraform/main.tf` – Terraform block (incl. backend), provider, IAM, Lambda, API Gateway, outputs. Single file.
- `cmd/ingest/main.go` – Lambda handler (healthz is a route, not a folder)
- `go.mod` – add `github.com/aws/aws-lambda-go`
- `Makefile` – target `build-ingest` for build + zip
- `.github/workflows/ci.yml` – CI pipeline
- This doc: `docs/phase-0-minimal-deploy.md`

**Generated / local:**

- `terraform/.terraform/` (after init)
- `terraform/.terraform.lock.hcl` (optional to commit)
- State in S3 (no local tfstate after migration)
- `build/bootstrap.zip` (build artifact)

**Commands you run:**

- `aws configure`
- `aws sts get-caller-identity`
- Create S3 bucket (+ DynamoDB) for state
- `terraform init` (in terraform/); migrate state when prompted
- `make build-ingest`
- `terraform plan` / `terraform apply` (in terraform/)
- `curl "$(terraform -chdir=terraform output -raw api_url)/healthz"`

---

## After Phase 0

You'll have:

- A working path from HTTP request to Go Lambda and back.
- Terraform that creates API Gateway + Lambda + IAM.
- A build and zip process for the Go Lambda.
- Remote state in S3 (shared across machines and CI).
- CI pipeline that runs build, test, lint, and `terraform plan` on every push.

Next you can add the real ingest (e.g. `POST /webhook`) and later SQS and the worker Lambda, reusing the same patterns.
