#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cluster_root="$repo_root/infrastructure/envs/mycaresignals/sre-lab/cluster"
backend_config="${SRE_LAB_CLUSTER_BACKEND_CONFIG:-$cluster_root/backend.hcl}"
tfvars_file="${SRE_LAB_CLUSTER_TFVARS:-$cluster_root/terraform.tfvars}"

"$repo_root/scripts/sre-lab/check-account.sh"

if ! command -v terraform >/dev/null 2>&1; then
  echo "Terraform is required to plan the SRE lab cluster." >&2
  exit 69
fi
if [[ ! -f "$backend_config" ]]; then
  echo "Missing cluster backend config: copy backend.hcl.example to backend.hcl." >&2
  exit 66
fi
if [[ ! -f "$tfvars_file" ]]; then
  echo "Missing cluster variables: copy terraform.tfvars.example to terraform.tfvars." >&2
  exit 66
fi

terraform -chdir="$cluster_root" init -input=false -backend-config="$backend_config"
terraform -chdir="$cluster_root" validate
terraform -chdir="$cluster_root" plan \
  -input=false \
  -lock-timeout=5m \
  -var-file="$tfvars_file" \
  -var="aws_account_id=${SRE_LAB_ACCOUNT_ID}"
