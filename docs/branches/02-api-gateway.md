# Branch 02: API Gateway

**Goal:** Add API Gateway HTTP API so the Lambda is reachable via public URL. Route `GET /healthz` invokes the Lambda. Phase 0 complete.

## What was added

- **API Gateway HTTP API** - `aws_apigatewayv2_api.main` (name: integration-engine, protocol_type: HTTP)
- **Stage** - `aws_apigatewayv2_stage.default` (exposes invoke URL)
- **Integration** - `aws_apigatewayv2_integration.healthz` (AWS_PROXY, links route to Lambda)
- **Route** - `aws_apigatewayv2_route.healthz` (GET /healthz)
- **Lambda permission** - `aws_lambda_permission.api_gateway` (allows API Gateway to invoke Lambda)
- **Output** - `api_url` (Terraform output for the base URL)
- **Provider region** - `provider "aws" { region = "eu-west-1" }` (explicit, not from aws configure)

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| API type | HTTP API (v2) | Simpler, cheaper than REST API. Matches Lambda proxy integration. |
| Stage name | default | Common convention. Invoke URL includes /default. |
| Output | api_url | Convenience; curl "$(terraform output -raw api_url)/healthz" |
| Region in TF | provider block | Reduces reliance on local aws configure; pipeline-friendly. |

## Steps (in order)

1. Add `provider "aws" { region = "eu-west-1" }` to main.tf.
2. Add `aws_apigatewayv2_api.main` (name: integration-engine, protocol_type: HTTP).
3. Add `aws_apigatewayv2_stage.default` (api_id, name: default, auto_deploy: true).
4. Add `aws_apigatewayv2_integration.healthz` (AWS_PROXY, integration_uri = Lambda invoke_arn, payload_format_version 2.0).
5. Add `aws_apigatewayv2_route.healthz` (route_key = "GET /healthz", target = integration ID).
6. Add `aws_lambda_permission.api_gateway` (allows apigateway.amazonaws.com to invoke Lambda; source_arn = API execution_arn).
7. Add `output "api_url"` = stage invoke_url.
8. Run `terraform plan && terraform apply`.
9. Verify: `curl "$(terraform -chdir=terraform output -raw api_url)/healthz"` → 200 and `{"status":"ok"}`.

## Files changed/added

- `terraform/main.tf` - Added API Gateway section (api, stage, integration, route, permission, output); added provider block with region.

## Next branch

**03-s3-backend-ci** - S3 backend for Terraform state, state migration, GitHub Actions CI (build, test, lint, terraform plan). Completes Phase 0.
