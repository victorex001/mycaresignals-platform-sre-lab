#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

require_command aws
require_command curl
require_command jq
verify_sre_lab_account "$repo_root"

if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
  echo "Temporary runner authorization is restricted to GitHub Actions." >&2
  exit 65
fi

cluster_name="${SRE_LAB_CLUSTER_NAME:-mycaresignals-sre-lab-eks}"
region="${AWS_REGION:-eu-west-2}"
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

runner_ip="$(curl --fail --silent --show-error https://checkip.amazonaws.com | tr -d '[:space:]')"
if [[ ! "$runner_ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
  echo "Could not resolve a valid IPv4 address for this GitHub runner." >&2
  exit 69
fi
runner_cidr="${runner_ip}/32"

original_cidrs="$(aws eks describe-cluster \
  --region "$region" \
  --name "$cluster_name" \
  --query 'cluster.resourcesVpcConfig.publicAccessCidrs' \
  --output json | jq -c '.')"
if jq -e 'index("0.0.0.0/0") != null' <<<"$original_cidrs" >/dev/null; then
  echo "Refusing an EKS endpoint that already allows 0.0.0.0/0." >&2
  exit 65
fi

updated_cidrs="$(jq -cn --argjson current "$original_cidrs" --arg runner "$runner_cidr" \
  '$current + [$runner] | unique')"
if [[ -n "${GITHUB_ENV:-}" ]]; then
  # Persist the restore target before making the network change so the final
  # workflow step can recover even if a later command fails.
  echo "SRE_LAB_ORIGINAL_PUBLIC_CIDRS=${original_cidrs}" >>"$GITHUB_ENV"
  echo "SRE_LAB_RUNNER_CIDR=${runner_cidr}" >>"$GITHUB_ENV"
fi
request="$(jq -cn \
  --arg name "$cluster_name" \
  --argjson cidrs "$updated_cidrs" \
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
      echo "EKS endpoint update ended with status ${update_status}." >&2
      exit 1
      ;;
  esac
  sleep 10
done
if [[ "${update_status:-}" != "Successful" ]]; then
  echo "Timed out waiting for temporary runner authorization." >&2
  exit 75
fi

echo "Temporarily authorized this reviewed GitHub runner at ${runner_cidr}."
