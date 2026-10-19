#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
lab_root="$repo_root/infrastructure/envs/mycaresignals/sre-lab"
foundation="$lab_root/foundation"
backend_config="${SRE_LAB_BACKEND_CONFIG:-$lab_root/backend.hcl}"
tfvars_file="${SRE_LAB_TFVARS:-$foundation/terraform.tfvars}"

"$repo_root/scripts/sre-lab/check-account.sh"

if ! command -v terraform >/dev/null 2>&1; then
  echo "Terraform is required to plan the SRE lab." >&2
  exit 69
fi
if [[ ! -f "$backend_config" ]]; then
  echo "Missing backend config: copy backend.hcl.example to backend.hcl and populate the lab state values." >&2
  exit 66
fi
if [[ ! -f "$tfvars_file" ]]; then
  echo "Missing foundation variables: copy terraform.tfvars.example to terraform.tfvars and populate lab-only values." >&2
  exit 66
fi

terraform -chdir="$foundation" init -input=false -backend-config="$backend_config"
terraform -chdir="$foundation" validate
terraform -chdir="$foundation" plan \
  -input=false \
  -lock-timeout=5m \
  -var-file="$tfvars_file" \
  -var="aws_account_id=${SRE_LAB_ACCOUNT_ID}"
