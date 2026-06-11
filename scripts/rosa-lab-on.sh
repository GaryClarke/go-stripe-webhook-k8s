#!/usr/bin/env bash
# Start ROSA lab: create cluster if missing, or show status + morning checklist.
# Run: make lab-on   or   ./scripts/rosa-lab-on.sh
# Does not wait for install (~30-45 min). Re-run when State is ready.

set -euo pipefail

CLUSTER_NAME="${ROSA_CLUSTER_NAME:-gc-rosa-lab}"

say() { printf '%s\n' "$*"; }

print_checklist() {
	local api="$1"
	local dns="$2"
	say ""
	say "=== Morning checklist (when State: ready) ==="
	if [ -n "$api" ]; then
		say "1. GitHub variable ROSA_API_URL (no spaces):"
		say "   ${api}"
	else
		say "1. GitHub variable ROSA_API_URL <- rosa describe cluster (API URL)"
	fi
	if [ -n "$dns" ]; then
		local apps_base="$dns"
		case "$dns" in
		"${CLUSTER_NAME}".*) apps_base="$dns" ;;
		*) apps_base="${CLUSTER_NAME}.${dns}" ;;
		esac
		say "2. Stripe webhook URL (edit existing endpoint):"
		say "   https://go-stripe-webhook-k8s-go-stripe-webhook.apps.${apps_base}/webhooks/stripe"
	else
		say "2. Stripe webhook URL <- oc get route after deploy"
	fi
	say "3. Fresh cluster only:"
	say "   rosa create idp --cluster=${CLUSTER_NAME} --type=htpasswd --name=lab-users --username=garyc --password='...'"
	say "   rosa grant user cluster-admin --user=garyc --cluster=${CLUSTER_NAME}"
	say "   oc login <API-URL> -u garyc -p '...' && oc new-project go-stripe-webhook"
	say "   secrets + oc apply -k k8s/overlays/rosa  (or push main if CI deploys)"
	say "4. make lab-status"
}

say "=== ROSA lab on (${CLUSTER_NAME}) ==="

if ! command -v rosa >/dev/null 2>&1; then
	say "ERROR: rosa CLI not found (brew install rosa-cli)."
	exit 1
fi

json_cluster() {
	rosa describe cluster -c "$CLUSTER_NAME" -o json 2>/dev/null || true
}

cluster_json="$(json_cluster)"
if [ -z "$cluster_json" ]; then
	say "No cluster found. Creating (install ~30-45 min)..."
	rosa create cluster --cluster-name="$CLUSTER_NAME" --sts --mode auto
	say ""
	say "Watch: rosa describe cluster -c ${CLUSTER_NAME}"
	say "      rosa logs install -c ${CLUSTER_NAME} --watch"
	print_checklist "" ""
	exit 0
fi

state="$(printf '%s' "$cluster_json" | python3 -c "import json,sys; print(json.load(sys.stdin).get('state',''))")"
api_url="$(printf '%s' "$cluster_json" | python3 -c "import json,sys; print(json.load(sys.stdin).get('api',{}).get('url','') or '')")"
dns="$(printf '%s' "$cluster_json" | python3 -c "import json,sys; print(json.load(sys.stdin).get('dns',{}).get('base_domain','') or '')")"

say "State: ${state}"
if [ -n "$dns" ] && [ "$dns" != "Not ready" ]; then
	case "$dns" in
	"${CLUSTER_NAME}".*) say "DNS: ${dns}" ;;
	*) say "DNS: ${CLUSTER_NAME}.${dns}" ;;
	esac
fi
if [ -n "$api_url" ]; then
	say "API URL: ${api_url}"
fi

case "$state" in
ready)
	say ""
	say "Cluster is ready. Run setup if this is a fresh cluster (IDP, project, secrets)."
	print_checklist "$api_url" "$dns"
	;;
installing|waiting|pending|validating)
	say ""
	say "Install in progress. Re-run: make lab-on  (or rosa describe cluster -c ${CLUSTER_NAME})"
	print_checklist "$api_url" "$dns"
	;;
uninstalling)
	say ""
	say "Cluster is uninstalling. Wait until gone, then: make lab-on"
	;;
*)
	say ""
	say "Unexpected state. Check: rosa describe cluster -c ${CLUSTER_NAME}"
	;;
esac
