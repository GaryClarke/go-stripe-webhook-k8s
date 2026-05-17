# -----------------------------------------------------------------------------
# ECR + GitHub Actions OIDC (branch 10)
#
# Creates:
#   - ECR repository for the go-stripe-webhook-k8s image
#   - IAM OIDC identity provider for token.actions.githubusercontent.com
#   - IAM role GitHub Actions can assume (narrow trust: one repo + branch ref)
#   - Inline policy: push only to this ECR repository
#
# If the OIDC provider already exists in this AWS account (another repo set it
# up first), remove the aws_iam_openid_connect_provider resource here and use a
# data "aws_iam_openid_connect_provider" block instead, or import:
#   terraform import aws_iam_openid_connect_provider.github \
#     arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com
# -----------------------------------------------------------------------------

# --- BEGINNER: What is this file? ---
# Terraform describes *desired state*. Each `resource` block tells AWS to create
# (or update) one thing. `terraform apply` sends this to AWS; AWS creates ECR,
# IAM roles, etc. Re-running apply is safe: Terraform reconciles drift.

# `data` blocks *read* existing or external info (no new AWS object). We use
# this to fetch GitHub's TLS certificate so AWS trusts the right OIDC endpoint.
# OIDC = OpenID Connect: GitHub's CI proves "this job is org/repo@ref" via a JWT;
# AWS exchanges that for temporary credentials via this provider + IAM role.
data "tls_certificate" "github_oidc" {
  url = "https://token.actions.githubusercontent.com"
}

# ECR = Elastic Container Registry: AWS-hosted Docker image storage (like Docker Hub).
# CI will `docker push` here after it assumes the IAM role below.
resource "aws_ecr_repository" "app" {
  name = var.ecr_repository_name
  # MUTABLE: allow retagging the same tag (e.g. `latest`). IMMUTABLE is stricter for prod.
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true # AWS scans pushed images for known CVEs (basic supply-chain hygiene)
  }
}

# Registers GitHub as an "identity provider" in IAM: *who* may call AssumeRoleWithWebIdentity.
# One provider per AWS account for github.com's issuer URL is typical; second stacks import it.
resource "aws_iam_openid_connect_provider" "github" {
  url = "https://token.actions.githubusercontent.com"

  # JWT `aud` (audience) AWS expects when GitHub requests credentials.
  client_id_list = [
    "sts.amazonaws.com",
  ]

  # Thumbprint ties this provider to GitHub's TLS cert. We pull it from the
  # data source above instead of hard-coding so cert rotations are less painful.
  thumbprint_list = [
    data.tls_certificate.github_oidc.certificates[0].sha1_fingerprint,
  ]
}

# IAM *role* = named bucket of permissions. The *trust policy* (assume_role_policy)
# says *only* GitHub Actions matching our repo+ref can assume this role.
# The *role policy* (next resource) says what AWS APIs that role may call once assumed.
resource "aws_iam_role" "github_actions_ecr" {
  name = var.github_actions_role_name

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = "sts:AssumeRoleWithWebIdentity" # web identity = OIDC, not long-lived keys
        Principal = {
          Federated = aws_iam_openid_connect_provider.github.arn
        }
        Condition = {
          StringEquals = {
            # Must match the token audience we configured on the OIDC provider.
            "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
            # NARROW TRUST: only workflows from this repo on this git ref (e.g. main branch).
            # Change github_repository / github_branch_ref in variables.tf to widen or restrict.
            "token.actions.githubusercontent.com:sub" = "repo:${var.github_repository}:ref:${var.github_branch_ref}"
          }
        }
      },
    ]
  })
}

# Inline policy attached to the role: *what* the role can do in AWS (here: ECR push only).
resource "aws_iam_role_policy" "github_actions_ecr_push" {
  name = "ecr-push-${var.ecr_repository_name}"
  role = aws_iam_role.github_actions_ecr.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "GetAuthorizationToken"
        Effect = "Allow"
        # Global ECR API: returns a short-lived login token for `docker login` / buildx.
        Action   = "ecr:GetAuthorizationToken"
        Resource = "*"
      },
      {
        Sid    = "PushImages"
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:PutImage",
          "ecr:InitiateLayerUpload",
          "ecr:UploadLayerPart",
          "ecr:CompleteLayerUpload",
        ]
        # Scoped to *this* repository only — the role cannot push to other ECR repos.
        Resource = aws_ecr_repository.app.arn
      },
    ]
  })
}
