# -----------------------------------------------------------------------------
# Input variables (shared across the stack)
# -----------------------------------------------------------------------------

variable "aws_region" {
  description = "AWS region for this stack (ECR and related APIs). Match your backend bucket region and where you want the image registry."
  type        = string
  default     = "eu-west-1"
}

variable "ecr_repository_name" {
  description = "Name of the ECR repository (image pushes target this name in the account/region)."
  type        = string
  default     = "go-stripe-webhook-k8s"
}

variable "github_repository" {
  description = "GitHub repository in the form ORG/REPO (must match the workflow repository for OIDC 'sub' claim)."
  type        = string
  default     = "GaryClarke/go-stripe-webhook-k8s"
}

variable "github_branch_ref" {
  description = "Git ref allowed to assume the role (GitHub OIDC 'sub' suffix). Narrow scope; use refs/heads/main for prod-like pushes only."
  type        = string
  default     = "refs/heads/main"
}

variable "github_actions_role_name" {
  description = "IAM role name GitHub Actions will assume (must be unique in the AWS account)."
  type        = string
  default     = "go-stripe-webhook-k8s-github-actions-ecr"
}
