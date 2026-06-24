# -----------------------------------------------------------------------------
# Input variables (RDS stack)
# -----------------------------------------------------------------------------

variable "aws_region" {
  description = "AWS region for RDS and ROSA lab resources. Must match the S3 backend region (eu-west-1)."
  type        = string
  default     = "eu-west-1"
}

variable "rosa_vpc_id" {
  description = "VPC for gc-rosa-lab. OCM JSON often omits .aws.vpc_id — discover via AWS CLI (Machine CIDR 10.0.0.0/16 or Name tag *-vpc). Changes on cluster recreate."
  type        = string
}

variable "rosa_private_subnet_ids" {
  description = "Private subnet IDs in that VPC (MapPublicIpOnLaunch=false). Step 3 may add a second AZ subnet for RDS subnet group."
  type        = list(string)
}

variable "rds_secondary_subnet_cidr" {
  description = "Unused CIDR in the ROSA VPC for a second private subnet (RDS needs 2 AZs). Lab default avoids existing 10.0.0.0/18 and 10.0.64.0/18."
  type        = string
  default     = "10.0.128.0/18"
}

variable "rds_secondary_subnet_az" {
  description = "AZ for the extra private subnet (must differ from the ROSA private subnet AZ)."
  type        = string
  default     = "eu-west-1b"
}

variable "db_identifier" {
  description = "RDS instance identifier (AWS console name)."
  type        = string
  default     = "go-stripe-webhook-rds"
}

variable "db_name" {
  description = "Initial database name (Goose migrations target this DB)."
  type        = string
  default     = "stripe_webhook"
}

variable "db_username" {
  description = "Master username for the RDS instance."
  type        = string
  default     = "webhook"
}

variable "db_master_password" {
  description = "Master password — set in local terraform.tfvars only; never commit."
  type        = string
  sensitive   = true
}

variable "db_instance_class" {
  description = "RDS instance size (lab: db.t4g.micro)."
  type        = string
  default     = "db.t4g.micro"
}

variable "db_allocated_storage_gb" {
  description = "Initial storage in GB."
  type        = number
  default     = 20
}
