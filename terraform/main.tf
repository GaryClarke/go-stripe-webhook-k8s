# -----------------------------------------------------------------------------
# Terraform and provider configuration
# -----------------------------------------------------------------------------

terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket         = "integration-engine-terraform-state"
    key            = "integration-engine/terraform.tfstate"
    region         = "eu-west-1"
    dynamodb_table = "terraform-locks"
  }
}

provider "aws" {
  region = "eu-west-1"
}

# -----------------------------------------------------------------------------
# IAM: Lambda execution role
# -----------------------------------------------------------------------------
# Role for the webhook ingest function (payments/billing events, e.g. Stripe).
# Only needs permission to write logs (AWSLambdaBasicExecutionRole).
# Who can *invoke* the Lambda is controlled later by aws_lambda_permission (API Gateway).

resource "aws_iam_role" "payments_events_ingest" {
  name = "integration-engine-payments-events-ingest"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

# Allow the role to write logs (required for Lambda to run).
resource "aws_iam_role_policy_attachment" "payments_events_ingest_logs" {
  role       = aws_iam_role.payments_events_ingest.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# -----------------------------------------------------------------------------
# Lambda: webhook ingest function (legacy; parent project).
# Build the zip with a Linux bootstrap binary if you need to refresh this stack.
# This fork's CI does not run make build-ingest; produce ../build/bootstrap.zip locally.
# -----------------------------------------------------------------------------

resource "aws_lambda_function" "payments_events_ingest" {
  function_name = "integration-engine-payments-events-ingest"
  role          = aws_iam_role.payments_events_ingest.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"

  filename         = "${path.module}/../build/bootstrap.zip"
  source_code_hash = filebase64sha256("${path.module}/../build/bootstrap.zip")

  logging_config {
    log_format = "Text"
  }
}

# -----------------------------------------------------------------------------
# API Gateway: HTTP API and GET /healthz route
# -----------------------------------------------------------------------------
# Public URL for the Lambda. API Gateway receives HTTP requests and invokes
# the Lambda. We need: API, stage (for URL), route, integration, and permission.

# The HTTP API (v2, not REST API v1).
resource "aws_apigatewayv2_api" "main" {
  name          = "integration-engine"
  protocol_type = "HTTP"
}

# A stage exposes the API at an invoke URL. "default" is a common name.
resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.main.id
  name        = "default"
  auto_deploy = true
}

# Route: GET /healthz forwards to the integration (which invokes the Lambda).
resource "aws_apigatewayv2_route" "healthz" {
  api_id    = aws_apigatewayv2_api.main.id
  route_key = "GET /healthz"
  target    = "integrations/${aws_apigatewayv2_integration.healthz.id}"
}

# Integration: when a route is hit, invoke this Lambda. AWS_PROXY passes
# the full request/response through (we don't transform it).
resource "aws_apigatewayv2_integration" "healthz" {
  api_id                 = aws_apigatewayv2_api.main.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.payments_events_ingest.invoke_arn
  payload_format_version = "2.0"
}

# Lambda resource policy: allow this API Gateway to invoke the Lambda.
# Without this, API Gateway gets 403 when it tries to call the Lambda.
resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.payments_events_ingest.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.main.execution_arn}/*/*"
}

# Output the public URL so you can curl it.
output "api_url" {
  value       = aws_apigatewayv2_stage.default.invoke_url
  description = "API Gateway invoke URL (e.g. curl https://<this>/healthz)"
}
