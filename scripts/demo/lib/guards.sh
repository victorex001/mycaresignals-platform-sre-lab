#!/usr/bin/env bash

set -euo pipefail

readonly MYCARESIGNALS_EXPECTED_ACCOUNT="121638211660"
readonly MYCARESIGNALS_EXPECTED_REGION="eu-west-2"
readonly MYCARESIGNALS_EXPECTED_ENV="demo"

readonly MYCARESIGNALS_DEMO_ECS_SERVICE="mycaresignals-demo-teapi"
readonly MYCARESIGNALS_DEMO_RDS_INSTANCE="mycaresignals-demo-pg"
readonly MYCARESIGNALS_DEMO_TARGET_GROUP="mycaresignals-demo-teapi-shared"
readonly MYCARESIGNALS_DEMO_MIGRATION_FAMILY="mycaresignals-demo-teapi-migration"
readonly MYCARESIGNALS_SHARED_ECS_CLUSTER="mycaresignals-prod-cluster"
readonly MYCARESIGNALS_SHARED_ALB="mycaresignals-prod-alb"

readonly MYCARESIGNALS_PROD_STAFF_ECS_SERVICE="mycaresignals-prod-teapi"
readonly MYCARESIGNALS_PROD_FEEDBACK_ECS_SERVICE="mycaresignals-prod-qrfeedback"
readonly MYCARESIGNALS_PROD_RDS_INSTANCE="mycaresignals-prod-pg"
readonly MYCARESIGNALS_PROD_FEEDBACK_RDS_INSTANCE="mycaresignals-prod-qrfeedback-pg"
readonly MYCARESIGNALS_PROD_STAFF_TARGET_GROUP="mycaresignals-prod-teapi"
readonly MYCARESIGNALS_PROD_FEEDBACK_TARGET_GROUP="mycaresignals-prod-qrfeedback-ap"
readonly MYCARESIGNALS_PROD_COGNITO_POOL_ID="eu-west-2_8ykyfY46a"
readonly MYCARESIGNALS_PROD_STAFF_CLOUDFRONT_ID="E3GRNC26HEEY2O"
readonly MYCARESIGNALS_PROD_FEEDBACK_CLOUDFRONT_ID="E2O1432XMAX8W6"
readonly MYCARESIGNALS_PROD_STAFF_BUCKET="mycaresignals-tefe-prod-121638211660-eu-west-2"
readonly MYCARESIGNALS_PROD_FEEDBACK_BUCKET="mycaresignals-qrfeedback-frontend-prod-121638211660-eu-west-2"

readonly MYCARESIGNALS_DEMO_CLOUDFRONT_ID="E3IYP9AFMHY0L5"
readonly MYCARESIGNALS_DEMO_FRONTEND_BUCKET="mycaresignals-tefe-demo-121638211660-eu-west-2"
readonly MYCARESIGNALS_DEMO_COGNITO_POOL_ID="eu-west-2_sLrLz6IaF"
readonly MYCARESIGNALS_DEMO_API_CERTIFICATE_ARN="arn:aws:acm:eu-west-2:121638211660:certificate/1352acb4-9876-4ba3-941f-b5b82a2fe80a"
readonly MYCARESIGNALS_DEMO_DSN_SECRET="mycaresignals-demo/teapi/postgres-dsn"

readonly MYCARESIGNALS_AWS_CLI="${AWS_CLI:-aws}"

guard_error() {
  printf 'ERROR: %s\n' "$*" >&2
}

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    guard_error "required command not found: ${command_name}"
    return 1
  fi
}

require_demo_environment() {
  if [[ "${MYCARESIGNALS_ENV:-}" != "$MYCARESIGNALS_EXPECTED_ENV" ]]; then
    guard_error "MYCARESIGNALS_ENV must be explicitly set to demo"
    return 1
  fi
}

runtime_region() {
  local aws_region="${AWS_REGION:-}"
  local aws_default_region="${AWS_DEFAULT_REGION:-}"

  if [[ -n "$aws_region" && -n "$aws_default_region" && "$aws_region" != "$aws_default_region" ]]; then
    guard_error "AWS_REGION and AWS_DEFAULT_REGION disagree"
    return 1
  fi

  if [[ -n "$aws_region" ]]; then
    printf '%s\n' "$aws_region"
    return 0
  fi
  if [[ -n "$aws_default_region" ]]; then
    printf '%s\n' "$aws_default_region"
    return 0
  fi

  guard_error "AWS_REGION or AWS_DEFAULT_REGION must be explicitly set"
  return 1
}

require_runtime_region() {
  local region
  region="$(runtime_region)" || return 1
  if [[ "$region" != "$MYCARESIGNALS_EXPECTED_REGION" ]]; then
    guard_error "runtime region must be ${MYCARESIGNALS_EXPECTED_REGION}; received ${region}"
    return 1
  fi
}

aws_readonly() {
  "$MYCARESIGNALS_AWS_CLI" --no-cli-pager --region "$MYCARESIGNALS_EXPECTED_REGION" "$@"
}

require_aws_identity() {
  local account

  if ! account="$(aws_readonly sts get-caller-identity --query Account --output text 2>/dev/null)"; then
    guard_error "AWS identity inspection failed; credentials may be missing or expired"
    return 1
  fi
  if [[ "$account" != "$MYCARESIGNALS_EXPECTED_ACCOUNT" ]]; then
    guard_error "AWS account mismatch; expected ${MYCARESIGNALS_EXPECTED_ACCOUNT}, received ${account:-unknown}"
    return 1
  fi
}

require_demo_guards() {
  require_demo_environment || return 1
  require_runtime_region || return 1
  require_command "$MYCARESIGNALS_AWS_CLI" || return 1
  require_aws_identity || return 1
}

is_exact_demo_mutation_target() {
  local target="$1"
  case "$target" in
    "$MYCARESIGNALS_DEMO_ECS_SERVICE"|"$MYCARESIGNALS_DEMO_RDS_INSTANCE") return 0 ;;
    *) return 1 ;;
  esac
}

is_protected_production_or_shared_target() {
  local target="$1"

  case "$target" in
    "$MYCARESIGNALS_SHARED_ECS_CLUSTER"|"$MYCARESIGNALS_SHARED_ALB"|\
    "$MYCARESIGNALS_PROD_STAFF_ECS_SERVICE"|"$MYCARESIGNALS_PROD_FEEDBACK_ECS_SERVICE"|\
    "$MYCARESIGNALS_PROD_RDS_INSTANCE"|"$MYCARESIGNALS_PROD_FEEDBACK_RDS_INSTANCE"|\
    "$MYCARESIGNALS_PROD_STAFF_TARGET_GROUP"|"$MYCARESIGNALS_PROD_FEEDBACK_TARGET_GROUP"|\
    "$MYCARESIGNALS_PROD_COGNITO_POOL_ID"|\
    "$MYCARESIGNALS_PROD_STAFF_CLOUDFRONT_ID"|"$MYCARESIGNALS_PROD_FEEDBACK_CLOUDFRONT_ID"|\
    "$MYCARESIGNALS_PROD_STAFF_BUCKET"|"$MYCARESIGNALS_PROD_FEEDBACK_BUCKET"|\
    "app.mycaresignals.co.uk"|"api.mycaresignals.co.uk"|\
    "childvoice.mycaresignals.co.uk"|"visitorfeedback.mycaresignals.co.uk"|\
    "therapeutic-engine"|"therapeutic-engine-migrate"|"qr-feedback-api"|\
    "mycaresignals-terraform-state-new"|"mycaresignals-terraform-locks-new") return 0 ;;
    mycaresignals-prod-*|arn:aws:*:*:"$MYCARESIGNALS_EXPECTED_ACCOUNT":*:mycaresignals-prod-*|\
    *mycaresignals-prod-alb*|*mycaresignals-prod-cluster*) return 0 ;;
    *) return 1 ;;
  esac
}

assert_safe_demo_mutation_target() {
  local target="${1:-}"

  if [[ -z "$target" ]]; then
    guard_error "mutation target must not be empty"
    return 1
  fi
  if is_protected_production_or_shared_target "$target"; then
    guard_error "production/shared resource is never a demo mutation target: ${target}"
    return 1
  fi
  if ! is_exact_demo_mutation_target "$target"; then
    guard_error "resource is not in the exact demo mutation allowlist: ${target}"
    return 1
  fi
}

assert_read_only_shared_parent() {
  local target="$1"
  case "$target" in
    "$MYCARESIGNALS_SHARED_ECS_CLUSTER"|"$MYCARESIGNALS_SHARED_ALB") return 0 ;;
    *)
      guard_error "resource is not an allowlisted shared-parent reference: ${target}"
      return 1
      ;;
  esac
}

demo_lock_path() {
  printf '%s/mycaresignals-demo-lifecycle.lock\n' "${TMPDIR:-/tmp}"
}

acquire_demo_lifecycle_lock() {
  local operation="$1"
  local lock_path
  lock_path="$(demo_lock_path)"

  if ! mkdir "$lock_path" 2>/dev/null; then
    guard_error "another demo lifecycle operation holds ${lock_path}"
    return 1
  fi

  printf '%s\n' "$$" >"${lock_path}/pid"
  printf '%s\n' "$operation" >"${lock_path}/operation"
  export MYCARESIGNALS_DEMO_LOCK_PATH="$lock_path"
  export MYCARESIGNALS_DEMO_LOCK_OWNER_PID="$$"
  trap release_demo_lifecycle_lock EXIT INT TERM
}

demo_lifecycle_lock_is_owned() {
  local lock_path="${MYCARESIGNALS_DEMO_LOCK_PATH:-}"
  local owner_pid="${MYCARESIGNALS_DEMO_LOCK_OWNER_PID:-}"
  [[ -n "$lock_path" && -n "$owner_pid" && -f "${lock_path}/pid" ]] \
    && [[ "$(<"${lock_path}/pid")" == "$owner_pid" ]]
}

release_demo_lifecycle_lock() {
  local lock_path="${MYCARESIGNALS_DEMO_LOCK_PATH:-}"
  if [[ -n "$lock_path" && -d "$lock_path" ]]; then
    rm -f "${lock_path}/pid" "${lock_path}/operation"
    rmdir "$lock_path" 2>/dev/null || true
  fi
  unset MYCARESIGNALS_DEMO_LOCK_PATH
  unset MYCARESIGNALS_DEMO_LOCK_OWNER_PID
}
