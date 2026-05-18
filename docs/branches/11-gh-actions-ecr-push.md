# Branch 11: GitHub Actions ECR push (`11-gh-actions-ecr-push`)

**Goal:** **Milestone 3** CI step **(3)** from **[PLAN.md](../../PLAN.md)**: on every push to **`main`**, after **Go** gates pass, **build** the **`Dockerfile`** with **`docker buildx`** ( **`linux/amd64`** ), authenticate to **AWS** via **OIDC** (**AssumeRoleWithWebIdentity**), **log in** to **ECR**, and **push** tags **`${{ github.sha }}`** and **`latest`**. No long-lived AWS keys in **Secrets** - **role ARN** and optional region/repo name live in **Actions Variables**.

## What we added

- **`.github/workflows/ci.yaml`**: workflow-level **`permissions`** (**`contents: read`**, **`id-token: write`**); job **`push-ecr`** with **`needs: [build, test, lint]`**; **`aws-actions/configure-aws-credentials@v4`** (**`role-to-assume: ${{ vars.AWS_ROLE_ARN }}`**); **`aws-actions/amazon-ecr-login@v2`** (**`id: ecr`**); **`docker/setup-buildx-action@v3`**; **`docker buildx build ... --push .`** with **`--provenance=false`** and **`--sbom=false`** for a straightforward first push.
- **Header comments** in the workflow: required **`AWS_ROLE_ARN`** variable (from **`terraform output -raw github_actions_role_arn`**), optional **`AWS_REGION`** / **`ECR_REPOSITORY`**, and reminder that **IAM** trust must match **this repo** + **`refs/heads/main`**.

## Files changed (high level)

- **`.github/workflows/ci.yaml`**

## Prerequisites (GitHub repo settings)

Under **Settings - Secrets and variables - Actions - Variables**, set at least:

- **`AWS_ROLE_ARN`** = output of **`terraform output -raw github_actions_role_arn`** (from **`infra/terraform`**, after **`apply`**).

Optional: **`AWS_REGION`**, **`ECR_REPOSITORY`** (defaults in the workflow match **`infra/terraform/variables.tf`**).

## How to verify

1. Merge to **`main`** (or push a commit to **`main`**) and open **Actions** - workflow **CI** should show **build**, **test**, **lint**, then **push-ecr** green.
2. **ECR** ( **`eu-west-1`** or your region): repository **`go-stripe-webhook-k8s`** should show a new image tag matching the commit SHA (and **`latest`** updated).
3. If **assume role** fails: confirm **`AWS_ROLE_ARN`**, **GitHub org/repo** and branch match **Terraform** **`github_repository`** / **`github_branch_ref`**, and **`id-token: write`** is present.

## Follow-ups

- **Milestone 4:** **`k8s/`** manifests using this image (registry URL from **`terraform output -raw ecr_repository_url`**).
- Optional: drop **`:latest`** in CI for immutable tags only; add **multi-arch** (**`arm64`**) if nodes need it.
