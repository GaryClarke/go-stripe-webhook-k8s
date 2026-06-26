#!/usr/bin/env bash
# ROSA lab session health check (gc-rosa-lab).
# Run: make lab-status   or   ./scripts/lab-status.sh
# Does not read or print secrets. Exits 0 when cluster + app look OK; 1 when user action needed.

set -euo pipefail

CLUSTER_NAME="${ROSA_CLUSTER_NAME:-gc-rosa-lab}"
OC_PROJECT="${OC_PROJECT_NAME:-go-stripe-webhook}"
# DNS suffix (e.g. upgg) changes when the cluster is deleted and recreated; override via env if needed.
API_URL="${ROSA_API_URL:-https://api.gc-rosa-lab.upgg.p1.openshiftapps.com:6443}"
ROUTE_HOST="${ROSA_ROUTE_HOST:-go-stripe-webhook-k8s-go-stripe-webhook.apps.gc-rosa-lab.upgg.p1.openshiftapps.com}"
READYZ_URL="https://${ROUTE_HOST}/readyz"
DEPLOYMENT="${K8S_DEPLOYMENT_NAME:-go-stripe-webhook-k8s}"

need_user=0

say() { printf '%s\n' "$*"; }
need() {
	need_user=1
	say ""
	say ">>> YOU: $1"
}

say "=== ROSA lab status (${CLUSTER_NAME}) ==="

# --- Red Hat / rosa ---
if ! command -v rosa >/dev/null 2>&1; then
	need "Install rosa CLI (e.g. brew install rosa-cli)."
else
	if rosa_out="$(rosa list clusters 2>&1)"; then
		say "rosa: OK"
		echo "$rosa_out" | grep -E "^ID|${CLUSTER_NAME}" || true
		if ! echo "$rosa_out" | grep -q "${CLUSTER_NAME}.*ready"; then
			state_line="$(echo "$rosa_out" | grep "${CLUSTER_NAME}" || true)"
			if echo "$state_line" | grep -qiE 'hibernat|stop|pending|install|error|uninstall'; then
				need "Cluster not ready. Try: rosa start cluster --cluster=${CLUSTER_NAME} (if your CLI supports it) or start/hibernate from console.redhat.com. Then re-run: make lab-status"
			elif [ -n "$state_line" ]; then
				need "Cluster state is not 'ready': ${state_line}. Wait or fix before working."
			fi
		fi
	else
		if echo "$rosa_out" | grep -qiE 'token|expired|login|authentication'; then
			need "Refresh Red Hat SSO, then re-run make lab-status:"
			say "    rosa login --use-auth-code"
		else
			need "rosa failed:"
			say "$rosa_out"
		fi
	fi
fi

# --- OpenShift / oc ---
if ! command -v oc >/dev/null 2>&1; then
	need "Install oc CLI (e.g. brew install openshift-cli)."
else
	if oc_user="$(oc whoami 2>&1)"; then
		say "oc: logged in as ${oc_user}"
		oc project "${OC_PROJECT}" >/dev/null 2>&1 || need "Switch project: oc project ${OC_PROJECT}"
	else
		need "Log in to the cluster (use your htpasswd user, e.g. garyc):"
		say "    oc login ${API_URL} -u garyc -p '<your-lab-password>'"
		say "    oc project ${OC_PROJECT}"
	fi

	if oc whoami >/dev/null 2>&1; then
		say ""
		say "Pods:"
		pods="$(oc get pods -l app="${DEPLOYMENT}" 2>&1)" || pods=""
		say "$pods"
		if echo "$pods" | grep -q ImagePullBackOff; then
			need "ECR pull token expired. Refresh secret, then rollout (AWS CLI must work):"
			say "    oc delete secret ecr-registry --ignore-not-found"
			say "    oc create secret docker-registry ecr-registry \\"
			say "      --docker-server=234290944523.dkr.ecr.eu-west-1.amazonaws.com \\"
			say "      --docker-username=AWS \\"
			say "      --docker-password=\$(aws ecr get-login-password --region eu-west-1)"
			say "    oc rollout restart deployment/${DEPLOYMENT}"
			say "    oc rollout status deployment/${DEPLOYMENT}"
		elif echo "$pods" | grep -qE 'CrashLoopBackOff|Error'; then
			need "Pod unhealthy. Inspect: oc describe pod -l app=${DEPLOYMENT} | tail -30"
		elif ! echo "$pods" | grep -qE '[0-9]+/[0-9]+.*Running'; then
			need "No Running pod for app=${DEPLOYMENT}. Check: oc get pods -l app=${DEPLOYMENT}"
		fi
	fi
fi

# --- Route host (DNS suffix changes after cluster recreate) ---
if oc whoami >/dev/null 2>&1; then
	if route_host="$(oc get route "${DEPLOYMENT}" -n "${OC_PROJECT}" -o jsonpath='{.spec.host}' 2>/dev/null)" && [ -n "$route_host" ]; then
		ROUTE_HOST="$route_host"
		READYZ_URL="https://${ROUTE_HOST}/readyz"
	fi
fi

# --- HTTP smoke ---
say ""
say "Route /readyz: ${READYZ_URL}"
if curl_out="$(curl -fsS --max-time 15 "${READYZ_URL}" 2>&1)"; then
	say "curl: OK  ${curl_out}"
else
	need "readyz failed. Check Route and pods:"
	say "    oc get route ${DEPLOYMENT}"
	say "    oc get pods -l app=${DEPLOYMENT}"
	say "    curl -v ${READYZ_URL}"
fi

say ""
if [ "$need_user" -eq 0 ]; then
	say "=== Lab OK — ready to work ==="
	say "Stripe logs: oc logs -l app=${DEPLOYMENT} --tail=30"
	exit 0
fi

say "=== Action needed (see >>> YOU above) ==="
exit 1
