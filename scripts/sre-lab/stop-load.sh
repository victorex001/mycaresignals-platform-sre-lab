#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

require_exact_confirmation \
  "${CONFIRM_SRE_LAB_STOP:-}" \
  "STOP_SRE_LAB_LOAD" \
  CONFIRM_SRE_LAB_STOP
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

cluster_name="${SRE_LAB_CLUSTER_NAME:-mycaresignals-sre-lab-eks}"
region="${AWS_REGION:-eu-west-2}"
load_node_group="${SRE_LAB_LOAD_NODE_GROUP:-load-generator-spot}"
app_node_group="${SRE_LAB_APP_NODE_GROUP:-application-on-demand}"

kubectl -n mycaresignals-load delete jobs \
  -l app.kubernetes.io/name=mycaresignals-load \
  --ignore-not-found=true \
  --wait=false

aws eks update-nodegroup-config \
  --region "$region" \
  --cluster-name "$cluster_name" \
  --nodegroup-name "$load_node_group" \
  --scaling-config "minSize=0,maxSize=20,desiredSize=0" >/dev/null
aws eks update-nodegroup-config \
  --region "$region" \
  --cluster-name "$cluster_name" \
  --nodegroup-name "$app_node_group" \
  --scaling-config "minSize=2,maxSize=10,desiredSize=2" >/dev/null

echo "Load jobs stopped; load nodes are scaling to zero and application nodes to two."
