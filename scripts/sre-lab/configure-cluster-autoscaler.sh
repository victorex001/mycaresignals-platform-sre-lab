#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

cluster_name="${SRE_LAB_CLUSTER_NAME:-mycaresignals-sre-lab-eks}"
region="${AWS_REGION:-eu-west-2}"
chart_version="${SRE_LAB_AUTOSCALER_CHART_VERSION:-9.59.0}"
image_tag="${SRE_LAB_AUTOSCALER_IMAGE_TAG:-v1.36.1}"

require_command helm
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

role_arn="${SRE_LAB_AUTOSCALER_ROLE_ARN:-}"
if [[ -z "$role_arn" ]]; then
  role_arn="$(aws iam get-role \
    --role-name mycaresignals-sre-lab-cluster-autoscaler \
    --query 'Role.Arn' \
    --output text)"
fi

if [[ ! "$role_arn" =~ ^arn:aws:iam::[0-9]{12}:role/mycaresignals-sre-lab-cluster-autoscaler$ ]]; then
  echo "Unexpected cluster-autoscaler role ARN: ${role_arn}" >&2
  exit 65
fi

helm repo add autoscaler https://kubernetes.github.io/autoscaler --force-update >/dev/null
helm repo update autoscaler >/dev/null
helm upgrade --install cluster-autoscaler autoscaler/cluster-autoscaler \
  --namespace kube-system \
  --version "$chart_version" \
  --set "autoDiscovery.clusterName=${cluster_name}" \
  --set "awsRegion=${region}" \
  --set "cloudProvider=aws" \
  --set "image.tag=${image_tag}" \
  --set "nodeSelector.workload=system" \
  --set "extraArgs.balance-similar-node-groups=true" \
  --set "extraArgs.expander=least-waste" \
  --set-string "rbac.serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn=${role_arn}" \
  --wait \
  --timeout 10m

kubectl -n kube-system rollout status deployment/cluster-autoscaler --timeout=10m
kubectl -n kube-system get deployment cluster-autoscaler -o wide
