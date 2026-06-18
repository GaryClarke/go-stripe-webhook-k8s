#!/usr/bin/env bash
# Stop ROSA lab billing: delete cluster (hibernate/stop not available on this account).
# Run: make lab-off   or   ./scripts/rosa-lab-off.sh
# After uninstall finishes, run the printed operator-roles / oidc cleanup (optional).

set -euo pipefail

CLUSTER_NAME="${ROSA_CLUSTER_NAME:-gc-rosa-lab}"

say() { printf '%s\n' "$*"; }

# Exit 1 with login instructions when Red Hat SSO token is missing or expired.
require_rosa_login() {
	local err
	if rosa whoami >/dev/null 2>&1; then
		return 0
	fi
	err="$(rosa whoami 2>&1)" || true
	if echo "$err" | grep -qiE 'token|expired|login|authentication|unavailable'; then
		say "ERROR: Not logged in to Red Hat (rosa session missing or expired)."
		say "Run:  rosa login --use-auth-code"
		say "Then: make lab-off"
		exit 1
	fi
	say "ERROR: rosa whoami failed:"
	say "$err"
	exit 1
}

say "=== ROSA lab off (${CLUSTER_NAME}) ==="

if ! command -v rosa >/dev/null 2>&1; then
	say "ERROR: rosa CLI not found (brew install rosa-cli)."
	exit 1
fi

require_rosa_login

CLUSTER_ID=""
describe_out=""
if describe_out="$(rosa describe cluster -c "$CLUSTER_NAME" -o json 2>&1)"; then
	rosa_out="$describe_out"
	CLUSTER_ID="$(printf '%s' "$rosa_out" | python3 -c "import json,sys; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || true)"
	state="$(printf '%s' "$rosa_out" | python3 -c "import json,sys; print(json.load(sys.stdin).get('state',''))" 2>/dev/null || true)"
	if [ "$state" = "uninstalling" ]; then
		say "Cluster is already uninstalling. Watch: rosa logs uninstall -c ${CLUSTER_NAME} --watch"
		exit 0
	fi
else
	if echo "$describe_out" | grep -qiE 'not found|does not exist|unable to find|404'; then
		say "Cluster '${CLUSTER_NAME}' not found (already off?)."
		exit 0
	fi
	say "ERROR: rosa describe cluster failed:"
	say "$describe_out"
	exit 1
fi

say "Deleting cluster (billing stops once nodes are gone, ~10-20 min)..."
rosa delete cluster --cluster="$CLUSTER_NAME" --yes

say ""
say "=== After uninstall completes ==="
if [ -n "$CLUSTER_ID" ]; then
	say "Optional IAM cleanup:"
	say "  rosa delete operator-roles -c ${CLUSTER_ID}"
	say "  rosa delete oidc-provider -c ${CLUSTER_ID}"
fi
say ""
say "Deploy ROSA on push to main will skip until the lab is back (expected)."
say "Watch: rosa logs uninstall -c ${CLUSTER_NAME} --watch"
