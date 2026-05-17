# -----------------------------------------------------------------------------
# AWS provider configuration
#
# The provider is the client Terraform uses to call AWS APIs. Region must match
# where you create regional resources (ECR lives in a region; IAM is global but
# the provider still needs a home region for many API calls).
#
# Splitting this from main.tf keeps "how we connect to AWS" separate from
# "what resources we create" (ECR repo, OIDC provider, IAM roles).
# -----------------------------------------------------------------------------

provider "aws" {
  # Region comes from variables.tf so you can override with -var or a .tfvars
  # file without editing this file.
  region = var.aws_region

  # default_tags are applied to *supported* resources automatically. Handy in
  # the AWS console: filter by Project or see that Terraform owns an object.
  # Not every resource type supports every tag; unsupported tags are ignored.
  default_tags {
    tags = {
      Project   = "go-stripe-webhook-k8s"
      ManagedBy = "terraform"
    }
  }
}
