# -----------------------------------------------------------------------------
# Terraform tooling and remote state
#
# This file is the usual place for three things:
#   1) Which Terraform CLI version you support
#   2) Which providers this stack needs (here: AWS)
#   3) Where state is stored (remote backend) so plans/applies are consistent
#
# Keep resource definitions (ECR, IAM, OIDC) in main.tf (or split .tf files).
# -----------------------------------------------------------------------------

terraform {
  # Minimum Terraform version. 1.x is fine for AWS + OIDC patterns used here.
  required_version = ">= 1.5"

  # aws: create ECR, IAM role, OIDC provider. tls: fetch GitHub OIDC TLS thumbprint.
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }

  # Remote state: Terraform remembers what it created (ECR repo, IAM role, etc.)
  # in S3 instead of only on your laptop. That lets you run plan/apply from
  # another machine or from CI later, and avoids losing track of resources.
  #
  # Important: use a STATE KEY that is NOT shared with other projects. The old
  # Lambda stack used a different key in the same bucket; this path is only
  # for go-stripe-webhook-k8s infra (OIDC + ECR + CI role).
  #
  # Before first `terraform init`:
  #   - Ensure the S3 bucket and (optional) DynamoDB lock table exist in AWS.
  #   - Adjust bucket, key, region, dynamodb_table below if yours differ.
  #
  # Alternative: comment out this entire `backend "s3"` block and run locally
  # with the default `local` backend until you are ready for S3; then
  # `terraform init -migrate-state` when adding the backend (Terraform will prompt).
  backend "s3" {
    bucket         = "integration-engine-terraform-state"
    key            = "go-stripe-webhook-k8s/infra/terraform.tfstate"
    region         = "eu-west-1"
    dynamodb_table = "terraform-locks"
    encrypt        = true
  }
}

# Note: the AWS provider block lives in providers.tf (uses var.aws_region).
