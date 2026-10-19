#!/usr/bin/env bash
set -euo pipefail

sre_lab_production_account_id="121638211660"

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Required command is unavailable: ${command_name}" >&2
    return 69
  fi
}

require_image_digest() {
  local image_uri="$1"
  local variable_name="$2"
  if [[ ! "$image_uri" =~ @sha256:[a-f0-9]{64}$ ]]; then
    echo "${variable_name} must be an immutable image URI ending in @sha256:<64 hex characters>." >&2
    return 64
  fi
}

verify_sre_lab_account() {
  local repo_root="$1"
  "$repo_root/scripts/sre-lab/check-account.sh"
}

verify_sre_lab_cluster() {
  local cluster_name="${SRE_LAB_CLUSTER_NAME:-mycaresignals-sre-lab-eks}"
  local region="${AWS_REGION:-eu-west-2}"

  require_command aws
  require_command kubectl

  if [[ ! "$cluster_name" =~ sre-lab ]]; then
    echo "Refusing cluster without sre-lab in its name: ${cluster_name}" >&2
    return 65
  fi

  local environment_tag
  environment_tag="$(aws eks describe-cluster \
    --region "$region" \
    --name "$cluster_name" \
    --query 'cluster.tags.Environment' \
    --output text)"
  if [[ "$environment_tag" != "sre-lab" ]]; then
    echo "EKS cluster is missing Environment=sre-lab." >&2
    return 65
  fi

  aws eks update-kubeconfig --region "$region" --name "$cluster_name" >/dev/null

  local namespace_environment namespace_data
  namespace_environment="$(kubectl get namespace mycaresignals-sre-lab -o jsonpath='{.metadata.labels.environment}' 2>/dev/null || true)"
  namespace_data="$(kubectl get namespace mycaresignals-sre-lab -o jsonpath='{.metadata.labels.data-classification}' 2>/dev/null || true)"
  if [[ -n "$namespace_environment" ]] && {
    [[ "$namespace_environment" != "sre-lab" ]] || [[ "$namespace_data" != "synthetic-only" ]];
  }; then
    echo "Existing application namespace is not labelled as a synthetic SRE lab." >&2
    return 65
  fi
}

require_exact_confirmation() {
  local actual="$1"
  local expected="$2"
  local variable_name="$3"
  if [[ "$actual" != "$expected" ]]; then
    echo "Set ${variable_name}=${expected} after reviewing the target and stop conditions." >&2
    return 64
  fi
}
