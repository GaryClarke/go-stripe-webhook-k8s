# -----------------------------------------------------------------------------
# Terraform tooling and remote state (RDS stack — separate from infra/terraform/)
#
# State key must differ from infra/terraform.tfstate so ECR/OIDC and RDS lifecycles
# stay independent (separate plan/apply/destroy).
# -----------------------------------------------------------------------------

terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket         = "integration-engine-terraform-state"
    key            = "go-stripe-webhook-k8s/infra/terraform/rds.tfstate"
    region         = "eu-west-1"
    dynamodb_table = "terraform-locks"
    encrypt        = true
  }
}
