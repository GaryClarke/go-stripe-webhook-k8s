# -----------------------------------------------------------------------------
# RDS for M8 idempotency (Phase 9)
#
# Separate Terraform root from infra/terraform/ (ECR + OIDC). Planned resources:
#   - data sources: ROSA VPC / private subnets
#   - aws_security_group: allow Postgres from ROSA worker nodes
#   - aws_db_instance: Postgres 16 (webhook idempotency ledger)
#
# Do not add ECR or GitHub OIDC here — those live in infra/terraform/.
# -----------------------------------------------------------------------------
