#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

seed_image="${SEED_IMAGE_URI:-}"
require_image_digest "$seed_image" SEED_IMAGE_URI
require_exact_confirmation \
  "${CONFIRM_SRE_LAB_SEED:-}" \
  "SRE_LAB_SYNTHETIC_ONLY" \
  CONFIRM_SRE_LAB_SEED
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

user_count="${SRE_LAB_USER_COUNT:-1000000}"
home_count="${SRE_LAB_HOME_COUNT:-10000}"
active_children="${SRE_LAB_ACTIVE_CHILDREN_PER_HOME:-4}"
historical_children="${SRE_LAB_HISTORICAL_CHILDREN_PER_HOME:-0}"
audit_events="${SRE_LAB_AUDIT_EVENT_COUNT:-100000}"

for value in "$user_count" "$home_count" "$active_children" "$historical_children" "$audit_events"; do
  if [[ ! "$value" =~ ^[0-9]+$ ]]; then
    echo "All seed counts must be non-negative integers." >&2
    exit 64
  fi
done

render_dir="$(mktemp -d)"
trap 'rm -rf "$render_dir"' EXIT
sed \
  -e "s|__SEED_IMAGE__|${seed_image}|g" \
  -e "s|__USER_COUNT__|${user_count}|g" \
  -e "s|__HOME_COUNT__|${home_count}|g" \
  -e "s|__ACTIVE_CHILDREN_PER_HOME__|${active_children}|g" \
  -e "s|__HISTORICAL_CHILDREN_PER_HOME__|${historical_children}|g" \
  -e "s|__AUDIT_EVENT_COUNT__|${audit_events}|g" \
  "$repo_root/infrastructure/envs/mycaresignals/sre-lab/kubernetes/seed-job.yaml.template" \
  >"$render_dir/seed-job.yaml"

kubectl -n mycaresignals-sre-lab delete job synthetic-seed --ignore-not-found --wait=true
kubectl apply -f "$render_dir/seed-job.yaml"
if ! kubectl -n mycaresignals-sre-lab wait --for=condition=complete job/synthetic-seed --timeout=3h; then
  kubectl -n mycaresignals-sre-lab logs job/synthetic-seed --all-containers=true || true
  exit 1
fi
kubectl -n mycaresignals-sre-lab logs job/synthetic-seed --all-containers=true
