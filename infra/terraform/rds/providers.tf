# -----------------------------------------------------------------------------
# AWS provider configuration (RDS stack)
#
# Region must match where ROSA workers and this RDS instance live (e.g. eu-west-1).
# Splitting this from main.tf keeps "how we connect to AWS" separate from
# "what resources we create" (VPC data, security groups, aws_db_instance, …).
# -----------------------------------------------------------------------------

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project   = "go-stripe-webhook-k8s"
      Stack     = "rds"
      ManagedBy = "terraform"
    }
  }
}
