#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

teapi_image="${TEAPI_IMAGE_URI:-}"
migrator_image="${MIGRATOR_IMAGE_URI:-}"
require_image_digest "$teapi_image" TEAPI_IMAGE_URI
require_image_digest "$migrator_image" MIGRATOR_IMAGE_URI
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

"$repo_root/scripts/sre-lab/configure-cluster-autoscaler.sh"
"$repo_root/scripts/sre-lab/configure-runtime-secrets.sh"

render_dir="$(mktemp -d)"
trap 'rm -rf "$render_dir"' EXIT

sed "s|__MIGRATOR_IMAGE__|${migrator_image}|g" \
  "$repo_root/infrastructure/envs/mycaresignals/sre-lab/kubernetes/migrate-job.yaml.template" \
  >"$render_dir/migrate-job.yaml"

kubectl -n mycaresignals-sre-lab delete job teapi-migrate --ignore-not-found --wait=true
kubectl apply -f "$render_dir/migrate-job.yaml"
if ! kubectl -n mycaresignals-sre-lab wait --for=condition=complete job/teapi-migrate --timeout=15m; then
  kubectl -n mycaresignals-sre-lab logs job/teapi-migrate --all-containers=true || true
  exit 1
fi

sed "s|__TEAPI_IMAGE__|${teapi_image}|g" \
  "$repo_root/infrastructure/envs/mycaresignals/sre-lab/kubernetes/teapi.yaml.template" \
  >"$render_dir/teapi.yaml"
kubectl apply -f "$render_dir/teapi.yaml"
kubectl -n mycaresignals-sre-lab rollout status deployment/teapi --timeout=15m

deployed_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
kubectl -n mycaresignals-sre-lab create configmap deployment-evidence \
  --from-literal="teapi_image=${teapi_image}" \
  --from-literal="migrator_image=${migrator_image}" \
  --from-literal="deployed_at=${deployed_at}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n mycaresignals-sre-lab get deployment,service,hpa,pdb,pods -o wide
