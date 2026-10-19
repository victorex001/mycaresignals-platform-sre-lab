#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

require_exact_confirmation \
  "${CONFIRM_SRE_LAB_COLLECT:-}" \
  "COLLECT_AND_STOP_SRE_LAB_LOAD" \
  CONFIRM_SRE_LAB_COLLECT
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

job_name="$(kubectl -n mycaresignals-load get configmap active-load-run \
  -o jsonpath='{.data.job_name}' 2>/dev/null || true)"
if [[ -z "$job_name" ]]; then
  echo "No recorded load run was found." >&2
  exit 66
fi

complete="$(kubectl -n mycaresignals-load get "job/${job_name}" \
  -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' 2>/dev/null || true)"
failed="$(kubectl -n mycaresignals-load get "job/${job_name}" \
  -o jsonpath='{.status.conditions[?(@.type=="Failed")].status}' 2>/dev/null || true)"
if [[ "$complete" != "True" && "$failed" != "True" ]]; then
  echo "Load job ${job_name} is still running; status must be Complete or Failed before collection." >&2
  exit 75
fi

result_dir="$(mktemp -d)"
trap 'rm -rf "$result_dir"' EXIT
result_file="$result_dir/${job_name}.log"
kubectl -n mycaresignals-load logs "job/${job_name}" \
  --all-containers=true \
  --prefix=true >"$result_file"

result_uri="not-uploaded"
if [[ -n "${SRE_LAB_RESULTS_BUCKET:-}" ]]; then
  result_uri="s3://${SRE_LAB_RESULTS_BUCKET}/load-tests/${job_name}.log"
  aws s3 cp "$result_file" "$result_uri" --sse aws:kms >/dev/null
fi

collected_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
kubectl -n mycaresignals-load create configmap last-load-result \
  --from-literal="job_name=${job_name}" \
  --from-literal="status=$([[ "$complete" == "True" ]] && echo complete || echo failed)" \
  --from-literal="collected_at=${collected_at}" \
  --from-literal="result_uri=${result_uri}" \
  --dry-run=client -o yaml | kubectl apply -f -

tail -n 100 "$result_file"
export CONFIRM_SRE_LAB_STOP=STOP_SRE_LAB_LOAD
"$repo_root/scripts/sre-lab/stop-load.sh"
