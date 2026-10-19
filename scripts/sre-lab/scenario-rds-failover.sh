#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

require_exact_confirmation "${CONFIRM_SRE_LAB_FAULT:-}" "FAILOVER_SYNTHETIC_RDS" CONFIRM_SRE_LAB_FAULT
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

rds_identifier="${SRE_LAB_RDS_IDENTIFIER:-mycaresignals-sre-lab-postgres}"
region="${AWS_REGION:-eu-west-2}"
multi_az="$(aws rds describe-db-instances \
  --region "$region" \
  --db-instance-identifier "$rds_identifier" \
  --query 'DBInstances[0].MultiAZ' \
  --output text)"
if [[ "$multi_az" != "True" ]]; then
  echo "RDS failover drill requires the synthetic database to be Multi-AZ." >&2
  exit 65
fi

aws rds reboot-db-instance \
  --region "$region" \
  --db-instance-identifier "$rds_identifier" \
  --force-failover >/dev/null
echo "Requested failover of the isolated synthetic RDS instance."
