# -----------------------------------------------------------------------------
# Outputs for operators and GitHub Actions
#
# Planned (after aws_db_instance):
#   - database_url (sensitive) → GitHub secret DATABASE_URL — never commit the value
#   - db_endpoint, db_port (non-sensitive, for debugging)
# -----------------------------------------------------------------------------
output "database_url" {
  description = "Postgres DSN for GitHub secret DATABASE_URL (terraform output -raw database_url)."
  value       = "postgres://${var.db_username}:${var.db_master_password}@${aws_db_instance.webhook.address}:${aws_db_instance.webhook.port}/${var.db_name}?sslmode=require"
  sensitive   = true
}

output "db_endpoint" {
  description = "RDS hostname (debugging)."
  value       = aws_db_instance.webhook.address
}

output "db_port" {
  description = "RDS port (debugging)."
  value       = aws_db_instance.webhook.port
}
