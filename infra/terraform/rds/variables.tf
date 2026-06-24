# -----------------------------------------------------------------------------
# Input variables (RDS stack)
# -----------------------------------------------------------------------------

variable "aws_region" {
  description = "AWS region for RDS and ROSA lab resources. Must match the S3 backend region (eu-west-1)."
  type        = string
  default     = "eu-west-1"
}
