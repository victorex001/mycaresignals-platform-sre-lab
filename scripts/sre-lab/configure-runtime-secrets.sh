#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/sre-lab/lib/guard.sh
source "$repo_root/scripts/sre-lab/lib/guard.sh"

require_command jq
verify_sre_lab_account "$repo_root"
verify_sre_lab_cluster

runtime_secret_arn="${SRE_LAB_RUNTIME_SECRET_ARN:-}"
if [[ -z "$runtime_secret_arn" ]]; then
  echo "SRE_LAB_RUNTIME_SECRET_ARN is required." >&2
  exit 64
fi

runtime_json="$(aws secretsmanager get-secret-value \
  --region "${AWS_REGION:-eu-west-2}" \
  --secret-id "$runtime_secret_arn" \
  --query SecretString \
  --output text)"

db_host="$(jq -er '.db_host' <<<"$runtime_json")"
db_port="$(jq -er '.db_port' <<<"$runtime_json")"
db_name="$(jq -er '.db_name' <<<"$runtime_json")"
db_username="$(jq -er '.db_username' <<<"$runtime_json")"
db_password="$(jq -er '.db_password' <<<"$runtime_json")"
dev_jwt_secret="$(jq -er '.dev_jwt_hmac_secret' <<<"$runtime_json")"
feedback_token="$(jq -er '.feedback_intake_service_token' <<<"$runtime_json")"
postgres_dsn="postgres://${db_username}:${db_password}@${db_host}:${db_port}/${db_name}?sslmode=require"

kubectl apply -f "$repo_root/infrastructure/envs/mycaresignals/sre-lab/kubernetes/base.yaml"

kubectl -n mycaresignals-sre-lab create secret generic teapi-runtime \
  --from-literal="POSTGRES_DSN=${postgres_dsn}" \
  --from-literal="DEV_JWT_HMAC_SECRET=${dev_jwt_secret}" \
  --from-literal="FEEDBACK_INTAKE_SERVICE_TOKEN=${feedback_token}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n mycaresignals-sre-lab create secret generic teapi-database \
  --from-literal="PGHOST=${db_host}" \
  --from-literal="PGPORT=${db_port}" \
  --from-literal="PGDATABASE=${db_name}" \
  --from-literal="PGUSER=${db_username}" \
  --from-literal="PGPASSWORD=${db_password}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n mycaresignals-load create secret generic load-runtime \
  --from-literal="DEV_JWT_HMAC_SECRET=${dev_jwt_secret}" \
  --dry-run=client -o yaml | kubectl apply -f -

unset runtime_json db_password dev_jwt_secret feedback_token postgres_dsn
echo "Runtime secrets configured in the two isolated SRE lab namespaces."
