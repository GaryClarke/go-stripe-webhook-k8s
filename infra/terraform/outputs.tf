# -----------------------------------------------------------------------------
# Values for GitHub Actions (branch 11) and operators.
# Use: terraform output -raw github_actions_role_arn
# -----------------------------------------------------------------------------

# --- BEGINNER: What are outputs? ---
# After `terraform apply`, Terraform prints these values. You (and CI) can read
# them with `terraform output` or wire them into another system. They do not
# create resources — they just *expose* attributes from resources in main.tf.

output "ecr_repository_url" {
  description = "Registry URL for docker push (no trailing slash path). Example tag: <url>:<sha>"
  value       = aws_ecr_repository.app.repository_url
}

output "ecr_repository_arn" {
  description = "ECR repository ARN (for cross-references or IAM elsewhere)."
  value       = aws_ecr_repository.app.arn
}

output "ecr_repository_name" {
  description = "Short repository name inside ECR."
  value       = aws_ecr_repository.app.name
}

# This is the ARN you pass to `aws-actions/configure-aws-credentials` in GHA
# (`role-to-assume`) so the workflow assumes this role via OIDC — no static keys.
output "github_actions_role_arn" {
  description = "Pass to aws-actions/configure-aws-credentials as role-to-assume (OIDC)."
  value       = aws_iam_role.github_actions_ecr.arn
}

output "github_oidc_provider_arn" {
  description = "IAM OIDC provider ARN (for debugging or imports)."
  value       = aws_iam_openid_connect_provider.github.arn
}
