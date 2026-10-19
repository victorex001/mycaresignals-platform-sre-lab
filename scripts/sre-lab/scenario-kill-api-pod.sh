#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

require_exact_confirmation "${CONFIRM_SRE_LAB_FAULT:-}" "KILL_ONE_API_POD" CONFIRM_SRE_LAB_FAULT
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

pod_name="$(kubectl -n mycaresignals-sre-lab get pods \
  -l app.kubernetes.io/name=teapi \
  --field-selector=status.phase=Running \
  -o jsonpath='{.items[0].metadata.name}')"
if [[ -z "$pod_name" ]]; then
  echo "No running TEAPI pod is available for the drill." >&2
  exit 1
fi

kubectl -n mycaresignals-sre-lab delete pod "$pod_name" --wait=false
kubectl -n mycaresignals-sre-lab rollout status deployment/teapi --timeout=10m
echo "Deleted one lab TEAPI pod and observed deployment recovery."
