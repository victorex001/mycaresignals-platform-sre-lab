#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

require_command aws
require_command jq
verify_sre_lab_account "$repo_root"

cluster_name="${SRE_LAB_CLUSTER_NAME:-mycaresignals-sre-lab-eks}"
region="${AWS_REGION:-eu-west-2}"
original_cidrs="${SRE_LAB_ORIGINAL_PUBLIC_CIDRS:-}"
if ! jq -e 'type == "array" and length > 0 and index("0.0.0.0/0") == null' \
  <<<"$original_cidrs" >/dev/null; then
  echo "Original restricted EKS CIDRs are unavailable; refusing an ambiguous restore." >&2
  exit 65
fi
if [[ ! "$cluster_name" =~ sre-lab ]]; then
  echo "Refusing cluster without sre-lab in its name: ${cluster_name}" >&2
  exit 65
fi
environment_tag="$(aws eks describe-cluster \
  --region "$region" \
  --name "$cluster_name" \
  --query 'cluster.tags.Environment' \
  --output text)"
if [[ "$environment_tag" != "sre-lab" ]]; then
  echo "EKS cluster is missing Environment=sre-lab." >&2
  exit 65
fi

request="$(jq -cn \
  --arg name "$cluster_name" \
  --argjson cidrs "$original_cidrs" \
  '{name:$name,resourcesVpcConfig:{endpointPublicAccess:true,endpointPrivateAccess:true,publicAccessCidrs:$cidrs}}')"
update_id="$(aws eks update-cluster-config \
  --region "$region" \
  --cli-input-json "$request" \
  --query 'update.id' \
  --output text)"

for _ in {1..90}; do
  update_status="$(aws eks describe-update \
    --region "$region" \
    --name "$cluster_name" \
    --update-id "$update_id" \
    --query 'update.status' \
    --output text)"
  case "$update_status" in
    Successful) break ;;
    Failed|Cancelled)
      echo "EKS endpoint restore ended with status ${update_status}." >&2
      exit 1
      ;;
  esac
  sleep 10
done
if [[ "${update_status:-}" != "Successful" ]]; then
  echo "Timed out restoring restricted EKS endpoint CIDRs." >&2
  exit 75
fi

current_cidrs="$(aws eks describe-cluster \
  --region "$region" \
  --name "$cluster_name" \
  --query 'cluster.resourcesVpcConfig.publicAccessCidrs' \
  --output json | jq -c 'sort')"
expected_cidrs="$(jq -c 'sort' <<<"$original_cidrs")"
if [[ "$current_cidrs" != "$expected_cidrs" ]]; then
  echo "EKS endpoint CIDR restoration did not match the original set." >&2
  exit 1
fi
echo "Restored the restricted EKS endpoint CIDR set."
