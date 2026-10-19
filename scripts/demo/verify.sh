#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/status.sh
source "${SCRIPT_DIR}/lib/status.sh"

pass_count=0
fail_count=0
demo_ecs_observed=false
demo_rds_observed=false
demo_target_observed=false

pass() {
  printf '%s: PASS%s\n' "$1" "${2:+ — $2}"
  pass_count=$((pass_count + 1))
}

fail() {
  printf '%s: FAIL%s\n' "$1" "${2:+ — $2}"
  fail_count=$((fail_count + 1))
}

section() {
  printf '\n%s\n' "$1"
}

require_demo_guards
if demo_lifecycle_lock_is_owned; then
  log_lifecycle_event "verify" "demo-environment" "lock-reused" "owned lifecycle lock"
else
  acquire_demo_lifecycle_lock "verify"
fi
log_lifecycle_event "verify" "demo-environment" "started" "read-only verification"

printf 'MYCARESIGNALS DEMO STATUS\n'

section "IDENTITY"
pass "Account" "$MYCARESIGNALS_EXPECTED_ACCOUNT"
pass "Region" "$MYCARESIGNALS_EXPECTED_REGION"
printf 'Environment: %s\n' "$MYCARESIGNALS_ENV"

section "STAFF"
if demo_ecs="$(describe_ecs_service "$MYCARESIGNALS_DEMO_ECS_SERVICE" 2>/dev/null)"; then
  read -r demo_ecs_status demo_desired demo_running demo_pending demo_task_definition <<<"$demo_ecs"
  printf 'ECS service: %s (%s)\n' "$MYCARESIGNALS_DEMO_ECS_SERVICE" "$demo_ecs_status"
  printf 'Running tasks: %s desired, %s running, %s pending\n' "$demo_desired" "$demo_running" "$demo_pending"
  printf 'Task definition: %s\n' "$demo_task_definition"
  demo_ecs_observed=true
  pass "ECS inspection"
else
  fail "ECS inspection" "$MYCARESIGNALS_DEMO_ECS_SERVICE not readable"
fi

if demo_rds="$(describe_rds_instance "$MYCARESIGNALS_DEMO_RDS_INSTANCE" 2>/dev/null)"; then
  read -r demo_rds_status demo_deletion_protection demo_rds_class demo_storage <<<"$demo_rds"
  printf 'RDS: %s (%s, %s GiB, deletion protection %s)\n' \
    "$demo_rds_status" "$demo_rds_class" "$demo_storage" "$demo_deletion_protection"
  demo_rds_observed=true
  pass "RDS inspection"
else
  fail "RDS inspection" "$MYCARESIGNALS_DEMO_RDS_INSTANCE not readable"
fi

if demo_target_health="$(target_health_summary "$MYCARESIGNALS_DEMO_TARGET_GROUP" 2>/dev/null)"; then
  demo_target_observed=true
  if [[ -z "$demo_target_health" || "$demo_target_health" == "None" ]]; then
    printf 'Target health: no registered targets (valid when demo is off)\n'
  else
    printf 'Target health: %s\n' "$demo_target_health"
  fi
  pass "Target-group inspection"
else
  fail "Target-group inspection" "$MYCARESIGNALS_DEMO_TARGET_GROUP not readable"
fi

if frontend="$(aws_readonly cloudfront get-distribution --id "$MYCARESIGNALS_DEMO_CLOUDFRONT_ID" --query 'Distribution.[Status,DistributionConfig.Enabled,DomainName]' --output text 2>/dev/null)"; then
  read -r frontend_status frontend_enabled frontend_domain <<<"$frontend"
  printf 'Frontend: CloudFront %s, enabled %s, %s\n' "$frontend_status" "$frontend_enabled" "$frontend_domain"
  if aws_readonly s3api head-bucket --bucket "$MYCARESIGNALS_DEMO_FRONTEND_BUCKET" >/dev/null 2>&1; then
    pass "Frontend storage" "$MYCARESIGNALS_DEMO_FRONTEND_BUCKET"
  else
    fail "Frontend storage" "$MYCARESIGNALS_DEMO_FRONTEND_BUCKET not readable"
  fi
else
  fail "Frontend" "$MYCARESIGNALS_DEMO_CLOUDFRONT_ID not readable"
fi

if cognito="$(aws_readonly cognito-idp describe-user-pool --user-pool-id "$MYCARESIGNALS_DEMO_COGNITO_POOL_ID" --query 'UserPool.[Name,Id,EstimatedNumberOfUsers]' --output text 2>/dev/null)"; then
  read -r cognito_name cognito_id cognito_users <<<"$cognito"
  printf 'Cognito: %s (%s), estimated users %s\n' "$cognito_name" "$cognito_id" "$cognito_users"
  if cognito_client_count="$(aws_readonly cognito-idp list-user-pool-clients --user-pool-id "$MYCARESIGNALS_DEMO_COGNITO_POOL_ID" --max-results 60 --query 'length(UserPoolClients)' --output text 2>/dev/null)" \
    && cognito_domain_status="$(aws_readonly cognito-idp describe-user-pool-domain --domain auth.demo.mycaresignals.co.uk --query 'DomainDescription.Status' --output text 2>/dev/null)" \
    && [[ "$cognito_client_count" -gt 0 && "$cognito_domain_status" == "ACTIVE" ]]; then
    pass "Cognito configuration" "${cognito_client_count} client(s), custom domain active"
  else
    fail "Cognito configuration" "client or custom domain missing"
  fi
else
  fail "Cognito configuration" "$MYCARESIGNALS_DEMO_COGNITO_POOL_ID not readable"
fi

if aws_readonly secretsmanager describe-secret --secret-id "$MYCARESIGNALS_DEMO_DSN_SECRET" --query ARN --output text >/dev/null 2>&1; then
  pass "Database secret" "exists; value not read"
else
  fail "Database secret" "required secret not found"
fi

if certificate="$(aws_readonly acm describe-certificate --certificate-arn "$MYCARESIGNALS_DEMO_API_CERTIFICATE_ARN" --query 'Certificate.[DomainName,Status]' --output text 2>/dev/null)"; then
  read -r certificate_domain certificate_status <<<"$certificate"
  printf 'API TLS: %s (%s)\n' "$certificate_domain" "$certificate_status"
  [[ "$certificate_domain" == "api.demo.mycaresignals.co.uk" && "$certificate_status" == "ISSUED" ]] \
    && pass "API certificate" \
    || fail "API certificate" "unexpected domain or status"
else
  fail "API certificate" "certificate not readable"
fi

if [[ "$demo_ecs_observed" == true && "$demo_rds_observed" == true && "$demo_target_observed" == true ]]; then
  if [[ "$demo_desired" -eq 0 && "$demo_running" -eq 0 && "$demo_pending" -eq 0 \
    && "$demo_rds_status" == "stopped" \
    && ( -z "$demo_target_health" || "$demo_target_health" == "None" ) ]]; then
    pass "Lifecycle state" "OFF and stable"
  elif [[ "$demo_desired" -gt 0 && "$demo_running" -eq "$demo_desired" && "$demo_pending" -eq 0 \
    && "$demo_rds_status" == "available" \
    && "$demo_target_health" =~ (^|[[:space:]])healthy([[:space:]]|$) \
    && ! "$demo_target_health" =~ (^|[[:space:]])(unhealthy|unused|draining|initial|unavailable)([[:space:]]|$) ]]; then
    pass "Lifecycle state" "ON and stable"
  else
    fail "Lifecycle state" "demo is mixed, transitional, or degraded"
  fi
fi

section "FEEDBACK"
printf 'Isolated demo environment: NO\n'
printf 'Architecture: production Feedback only; it is not exercised as demo\n'
pass "Production Feedback protected"

section "PRODUCTION PROTECTION"
if ecs_service_is_stable "$MYCARESIGNALS_PROD_STAFF_ECS_SERVICE"; then
  prod_staff="$(describe_ecs_service "$MYCARESIGNALS_PROD_STAFF_ECS_SERVICE")"
  read -r _status desired running pending _task <<<"$prod_staff"
  pass "Staff ECS" "desired ${desired}, running ${running}, pending ${pending}"
else
  fail "Staff ECS" "not stable"
fi

if ecs_service_is_stable "$MYCARESIGNALS_PROD_FEEDBACK_ECS_SERVICE"; then
  prod_feedback="$(describe_ecs_service "$MYCARESIGNALS_PROD_FEEDBACK_ECS_SERVICE")"
  read -r _status desired running pending _task <<<"$prod_feedback"
  pass "Feedback ECS" "desired ${desired}, running ${running}, pending ${pending}"
else
  fail "Feedback ECS" "not stable"
fi

rds_instance_is_available "$MYCARESIGNALS_PROD_RDS_INSTANCE" \
  && pass "Production RDS" "available" \
  || fail "Production RDS" "not available"
rds_instance_is_available "$MYCARESIGNALS_PROD_FEEDBACK_RDS_INSTANCE" \
  && pass "Production Feedback RDS" "available" \
  || fail "Production Feedback RDS" "not available"

alb_arn=""
listener_arn=""
if alb="$(shared_alb_details 2>/dev/null)"; then
  read -r alb_arn alb_state alb_dns <<<"$alb"
  if [[ "$alb_state" == "active" ]]; then
    pass "Shared ALB" "$MYCARESIGNALS_SHARED_ALB active"
  else
    fail "Shared ALB" "state ${alb_state}"
  fi
else
  fail "Shared ALB" "not readable"
fi

if production_target_is_healthy "$MYCARESIGNALS_PROD_STAFF_TARGET_GROUP"; then
  pass "Staff target health"
else
  fail "Staff target health" "non-healthy or absent target"
fi
if production_target_is_healthy "$MYCARESIGNALS_PROD_FEEDBACK_TARGET_GROUP"; then
  pass "Feedback target health"
else
  fail "Feedback target health" "non-healthy or absent target"
fi

if [[ -n "$alb_arn" ]] && listener_arn="$(shared_https_listener_arn "$alb_arn" 2>/dev/null)" \
  && [[ -n "$listener_arn" && "$listener_arn" != "None" ]]; then
  if rules="$(listener_rules_json "$listener_arn" 2>/dev/null)" \
    && listener_certificates="$(aws_readonly elbv2 describe-listener-certificates --listener-arn "$listener_arn" --output json 2>/dev/null)" \
    && grep -Fq 'api.mycaresignals.co.uk' <<<"$rules" \
    && grep -Fq '/api/v1/feedback' <<<"$rules" \
    && grep -Fq 'api.demo.mycaresignals.co.uk' <<<"$rules" \
    && grep -Fq "$MYCARESIGNALS_DEMO_API_CERTIFICATE_ARN" <<<"$listener_certificates"; then
    pass "Routing" "required rules and demo certificate attachment present"
  else
    fail "Routing" "required listener rules missing"
  fi
else
  fail "Routing" "HTTPS listener missing"
fi

section "SHARED PARENTS"
assert_read_only_shared_parent "$MYCARESIGNALS_SHARED_ALB" && pass "ALB mutation protection"
assert_read_only_shared_parent "$MYCARESIGNALS_SHARED_ECS_CLUSTER" && pass "ECS cluster mutation protection"

printf '\nChecks passed: %s\n' "$pass_count"
printf 'Checks failed: %s\n' "$fail_count"
if [[ "$fail_count" -eq 0 ]]; then
  printf '\nSAFE FOR DEMO LIFECYCLE OPERATIONS: YES\n'
  log_lifecycle_event "verify" "demo-environment" "passed" "all read-only checks passed"
  exit 0
fi

printf '\nSAFE FOR DEMO LIFECYCLE OPERATIONS: NO\n'
log_lifecycle_event "verify" "demo-environment" "failed" "${fail_count} read-only checks failed"
exit 1
