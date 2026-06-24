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
