#!/usr/bin/env bash
# Refresh ECR pull secret and restart the webhook Deployment (tokens expire ~12h).
# Run when: ImagePullBackOff, or before a deliberate rollout after long idle.
# Requires: aws CLI credentials, oc logged in, project go-stripe-webhook.

set -euo pipefail

ECR_SERVER="${ECR_SERVER:-234290944523.dkr.ecr.eu-west-1.amazonaws.com}"
AWS_REGION="${AWS_REGION:-eu-west-1}"
OC_PROJECT="${OC_PROJECT_NAME:-go-stripe-webhook}"
DEPLOYMENT="${K8S_DEPLOYMENT_NAME:-go-stripe-webhook-k8s}"
SECRET_NAME="${ECR_SECRET_NAME:-ecr-registry}"

oc project "${OC_PROJECT}"

oc delete secret "${SECRET_NAME}" --ignore-not-found
oc create secret docker-registry "${SECRET_NAME}" \
	--docker-server="${ECR_SERVER}" \
	--docker-username=AWS \
	--docker-password="$(aws ecr get-login-password --region "${AWS_REGION}")"

oc rollout restart "deployment/${DEPLOYMENT}"
oc rollout status "deployment/${DEPLOYMENT}"

echo "ECR secret refreshed and deployment restarted."
