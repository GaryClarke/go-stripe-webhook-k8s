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
data "aws_vpc" "rosa" {
  id = var.rosa_vpc_id
}

data "aws_subnet" "rosa_private" {
  for_each = toset(var.rosa_private_subnet_ids)
  id       = each.value
}

# Route table for the existing ROSA private subnet — reuse for the RDS subnet.
data "aws_route_table" "rosa_private" {
  subnet_id = var.rosa_private_subnet_ids[0]
}

resource "aws_subnet" "rds_secondary" {
  vpc_id            = data.aws_vpc.rosa.id
  cidr_block        = var.rds_secondary_subnet_cidr
  availability_zone = var.rds_secondary_subnet_az

  tags = {
    Name = "go-stripe-webhook-rds-private-${var.rds_secondary_subnet_az}"
  }
}

resource "aws_route_table_association" "rds_secondary" {
  subnet_id      = aws_subnet.rds_secondary.id
  route_table_id = data.aws_route_table.rosa_private.id
}

resource "aws_security_group" "rds" {
  name        = "go-stripe-webhook-rds"
  description = "Postgres from ROSA VPC workers"
  vpc_id      = data.aws_vpc.rosa.id

  ingress {
    description = "Postgres from VPC (ROSA workers)"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [data.aws_vpc.rosa.cidr_block]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_db_subnet_group" "webhook" {
  name = "go-stripe-webhook-rds"
  subnet_ids = concat(
    var.rosa_private_subnet_ids,
    [aws_subnet.rds_secondary.id],
  )

  tags = {
    Name = "go-stripe-webhook-rds"
  }
}

resource "aws_db_instance" "webhook" {
  identifier = var.db_identifier

  engine         = "postgres"
  engine_version = "16"
  instance_class = var.db_instance_class

  allocated_storage = var.db_allocated_storage_gb
  storage_type      = "gp3"

  db_name  = var.db_name
  username = var.db_username
  password = var.db_master_password

  db_subnet_group_name   = aws_db_subnet_group.webhook.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  # Lab: single-AZ, co-located with ROSA workers (eu-west-1a).
  availability_zone   = "eu-west-1a"
  publicly_accessible = false
  multi_az            = false

  backup_retention_period = 0
  skip_final_snapshot     = true
  deletion_protection     = false

  tags = {
    Name = var.db_identifier
  }
}
