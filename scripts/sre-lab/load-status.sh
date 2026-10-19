#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

job_name="$(kubectl -n mycaresignals-load get configmap active-load-run \
  -o jsonpath='{.data.job_name}' 2>/dev/null || true)"
if [[ -z "$job_name" ]]; then
  echo "No recorded load run was found."
  exit 0
fi

profile="$(kubectl -n mycaresignals-load get configmap active-load-run -o jsonpath='{.data.profile}')"
started_at="$(kubectl -n mycaresignals-load get configmap active-load-run -o jsonpath='{.data.started_at}')"
complete="$(kubectl -n mycaresignals-load get "job/${job_name}" -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' 2>/dev/null || true)"
failed="$(kubectl -n mycaresignals-load get "job/${job_name}" -o jsonpath='{.status.conditions[?(@.type=="Failed")].status}' 2>/dev/null || true)"
active="$(kubectl -n mycaresignals-load get "job/${job_name}" -o jsonpath='{.status.active}' 2>/dev/null || true)"

status="running"
if [[ "$complete" == "True" ]]; then
  status="complete"
elif [[ "$failed" == "True" ]]; then
  status="failed"
elif ! kubectl -n mycaresignals-load get "job/${job_name}" >/dev/null 2>&1; then
  status="missing"
fi

echo "job=${job_name} profile=${profile} started_at=${started_at} status=${status} active_pods=${active:-0}"
kubectl -n mycaresignals-load get "job/${job_name}" -o wide 2>/dev/null || true
kubectl -n mycaresignals-load get pods -l "job-name=${job_name}" -o wide 2>/dev/null || true
kubectl -n mycaresignals-sre-lab get deployment/teapi hpa/teapi -o wide 2>/dev/null || true
kubectl -n kube-system get deployment/cluster-autoscaler -o wide 2>/dev/null || true

if [[ "$status" == "failed" || "$status" == "missing" ]]; then
  exit 1
fi
