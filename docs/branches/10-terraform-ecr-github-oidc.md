# Branch 10: Terraform ECR + GitHub OIDC (`10-terraform-ecr-github-oidc`)

**Goal:** **Milestone 3** registry layer: replace legacy Lambda-era **`terraform/`** in this fork with a focused root under **`infra/terraform/`** - **ECR** repository, **GitHub OIDC** identity provider (issuer **`token.actions.githubusercontent.com`**), **IAM role** trusted for **AssumeRoleWithWebIdentity** from this repo/ref only, and an inline policy scoped to **ECR push** for that repository. Remote **S3** state on a **dedicated key** (not shared with the old Lambda stack).

## What we added

- **`infra/terraform/`**: **`versions.tf`** (backend, providers, **`terraform >= 1.5`**), **`providers.tf`**, **`variables.tf`**, **`main.tf`**, **`outputs.tf`**, **`.terraform.lock.hcl`** (committed for reproducible provider versions).
- **Resources:** **`aws_ecr_repository`** (mutable tags, **scan on push**), **`aws_iam_openid_connect_provider`** (thumbprint via **`data.tls_certificate`**), **`aws_iam_role`** + **`aws_iam_role_policy`** ( **`ecr:GetAuthorizationToken`** + push actions on the repo ARN only).
- **Removed:** **`terraform/main.tf`** (Lambda + API Gateway stub from the parent project; full stack lives elsewhere if still needed).
- **`.gitignore`:** allow **`.terraform.lock.hcl`** under **`infra/terraform`** (comment explains why).
- **`cursor-rules.md`:** **Terraform apply gate** before **`aac`** / **`mmp`** when **`infra/terraform/`** changes (**`plan`**, explicit **y/n** before **`apply`**).

## Files changed (high level)

- **`infra/terraform/**`** (new root), **`.gitignore`**, **`cursor-rules.md`**, delete **`terraform/main.tf`**.

## How to verify

Use the **Terraform root** (commands assume repo root):

```bash
cd infra/terraform
terraform init
terraform plan
```

After apply, state and outputs are available only from this directory (S3 backend):

```bash
terraform state list
terraform output -raw github_actions_role_arn
terraform output -raw ecr_repository_url
```

**Console:** **ECR** ( **`eu-west-1`** ) - repository **`go-stripe-webhook-k8s`** - **Scan frequency** should show **Scan on push**. **IAM** - role **`go-stripe-webhook-k8s-github-actions-ecr`**, OIDC provider **`token.actions.githubusercontent.com`**.

## Follow-ups

- **CI ECR push:** **[11-gh-actions-ecr-push.md](11-gh-actions-ecr-push.md)** (**`11-gh-actions-ecr-push`**) - **OIDC**, **`configure-aws-credentials`**, ECR login, **`docker buildx`** push ( **[PLAN.md](../../PLAN.md)** Milestone 3).
- If the **OIDC provider** already existed in this AWS account, **`apply`** would have failed; use import or a **`data`** block (see comment at top of **`infra/terraform/main.tf`**).
