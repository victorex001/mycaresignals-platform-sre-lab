#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

profile="${1:-smoke}"
loadgen_image="${LOADGEN_IMAGE_URI:-}"
load_node_group="${SRE_LAB_LOAD_NODE_GROUP:-load-generator-spot}"
app_node_group="${SRE_LAB_APP_NODE_GROUP:-application-on-demand}"
cluster_name="${SRE_LAB_CLUSTER_NAME:-mycaresignals-sre-lab-eks}"
region="${AWS_REGION:-eu-west-2}"

require_image_digest "$loadgen_image" LOADGEN_IMAGE_URI
require_exact_confirmation "${CONFIRM_SRE_LAB_RUN:-}" "RUN_SYNTHETIC_ONLY" CONFIRM_SRE_LAB_RUN
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

previous_job="$(kubectl -n mycaresignals-load get configmap active-load-run \
  -o jsonpath='{.data.job_name}' 2>/dev/null || true)"
if [[ -n "$previous_job" ]] && kubectl -n mycaresignals-load get "job/${previous_job}" >/dev/null 2>&1; then
  previous_complete="$(kubectl -n mycaresignals-load get "job/${previous_job}" \
    -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' 2>/dev/null || true)"
  previous_failed="$(kubectl -n mycaresignals-load get "job/${previous_job}" \
    -o jsonpath='{.status.conditions[?(@.type=="Failed")].status}' 2>/dev/null || true)"
  if [[ "$previous_complete" != "True" && "$previous_failed" != "True" ]]; then
    echo "Refusing to overlap active load job ${previous_job}. Stop or collect it first." >&2
    exit 75
  fi
fi

case "$profile" in
  smoke)
    shards=2
    load_nodes=2
    app_nodes=2
    deadline=900
    ;;
  baseline)
    shards=4
    load_nodes=3
    app_nodes=2
    deadline=2400
    ;;
  scale)
    shards=12
    load_nodes=8
    app_nodes=4
    deadline=9000
    ;;
  soak)
    require_exact_confirmation "${CONFIRM_SRE_LAB_SOAK:-}" "RUN_8_HOUR_10K_SOAK" CONFIRM_SRE_LAB_SOAK
    shards=20
    load_nodes=12
    app_nodes=6
    deadline=30600
    ;;
  *)
    echo "Profile must be smoke, baseline, scale, or soak." >&2
    exit 64
    ;;
esac

aws eks update-nodegroup-config \
  --region "$region" \
  --cluster-name "$cluster_name" \
  --nodegroup-name "$load_node_group" \
  --scaling-config "minSize=0,maxSize=20,desiredSize=${load_nodes}" >/dev/null
aws eks update-nodegroup-config \
  --region "$region" \
  --cluster-name "$cluster_name" \
  --nodegroup-name "$app_node_group" \
  --scaling-config "minSize=2,maxSize=10,desiredSize=${app_nodes}" >/dev/null

kubectl -n mycaresignals-sre-lab rollout status deployment/teapi --timeout=15m
kubectl -n mycaresignals-sre-lab get hpa teapi

run_stamp="$(date -u +%Y%m%d-%H%M%S)"
job_name="mcs-${profile}-${run_stamp}"
render_dir="$(mktemp -d)"
trap 'rm -rf "$render_dir"' EXIT

sed \
  -e "s|__JOB_NAME__|${job_name}|g" \
  -e "s|__PROFILE__|${profile}|g" \
  -e "s|__SHARDS__|${shards}|g" \
  -e "s|__ACTIVE_DEADLINE_SECONDS__|${deadline}|g" \
  -e "s|__LOADGEN_IMAGE__|${loadgen_image}|g" \
  "$repo_root/infrastructure/envs/mycaresignals/sre-lab/kubernetes/load-job.yaml.template" \
  >"$render_dir/load-job.yaml"

kubectl apply -f "$render_dir/load-job.yaml"
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
kubectl -n mycaresignals-load create configmap active-load-run \
  --from-literal="job_name=${job_name}" \
  --from-literal="profile=${profile}" \
  --from-literal="started_at=${started_at}" \
  --from-literal="active_deadline_seconds=${deadline}" \
  --dry-run=client -o yaml | kubectl apply -f -
echo "Started ${profile} load job ${job_name}."

if [[ "${SRE_LAB_WAIT_FOR_LOAD:-true}" == "true" ]]; then
  timeout_seconds=$((deadline + 600))
  if ! kubectl -n mycaresignals-load wait --for=condition=complete "job/${job_name}" --timeout="${timeout_seconds}s"; then
    kubectl -n mycaresignals-load get pods -l "job-name=${job_name}" -o wide || true
    kubectl -n mycaresignals-load logs "job/${job_name}" --all-containers=true --prefix=true || true
    exit 1
  fi

  result_file="$render_dir/${job_name}.log"
  kubectl -n mycaresignals-load logs "job/${job_name}" --all-containers=true --prefix=true >"$result_file"
  if [[ -n "${SRE_LAB_RESULTS_BUCKET:-}" ]]; then
    aws s3 cp "$result_file" "s3://${SRE_LAB_RESULTS_BUCKET}/load-tests/${job_name}.log" \
      --sse aws:kms >/dev/null
  fi
  tail -n 80 "$result_file"
fi
