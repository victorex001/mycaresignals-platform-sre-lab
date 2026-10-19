#!/usr/bin/env bash

set -euo pipefail

DEMO_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=guards.sh
source "${DEMO_LIB_DIR}/guards.sh"

log_lifecycle_event() {
  local operation="$1"
  local resource="$2"
  local status="$3"
  local message="${4:-}"
  local safe_message
  safe_message="${message//$'\n'/ }"

  printf 'timestamp=%s operation=%s environment=%s resource=%s status=%s message=%q\n' \
    "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
    "$operation" \
    "${MYCARESIGNALS_ENV:-unset}" \
    "$resource" \
    "$status" \
    "$safe_message" >&2
}

describe_ecs_service() {
  local service="$1"
  aws_readonly ecs describe-services \
    --cluster "$MYCARESIGNALS_SHARED_ECS_CLUSTER" \
    --services "$service" \
    --query 'services[0].[status,desiredCount,runningCount,pendingCount,taskDefinition]' \
    --output text
}

describe_rds_instance() {
  local instance="$1"
  aws_readonly rds describe-db-instances \
    --db-instance-identifier "$instance" \
    --query 'DBInstances[0].[DBInstanceStatus,DeletionProtection,DBInstanceClass,AllocatedStorage]' \
    --output text
}

target_group_arn() {
  local target_group="$1"
  aws_readonly elbv2 describe-target-groups \
    --names "$target_group" \
    --query 'TargetGroups[0].TargetGroupArn' \
    --output text
}

target_health_summary() {
  local target_group="$1"
  local arn
  arn="$(target_group_arn "$target_group")"
  aws_readonly elbv2 describe-target-health \
    --target-group-arn "$arn" \
    --query 'TargetHealthDescriptions[].TargetHealth.State' \
    --output text
}

shared_alb_details() {
  aws_readonly elbv2 describe-load-balancers \
    --names "$MYCARESIGNALS_SHARED_ALB" \
    --query 'LoadBalancers[0].[LoadBalancerArn,State.Code,DNSName]' \
    --output text
}

shared_https_listener_arn() {
  local alb_arn="$1"
  aws_readonly elbv2 describe-listeners \
    --load-balancer-arn "$alb_arn" \
    --query 'Listeners[?Port==`443`].ListenerArn | [0]' \
    --output text
}

listener_rules_json() {
  local listener_arn="$1"
  aws_readonly elbv2 describe-rules --listener-arn "$listener_arn" --output json
}

production_target_is_healthy() {
  local target_group="$1"
  local health
  health="$(target_health_summary "$target_group")"
  [[ -n "$health" && "$health" != "None" && ! "$health" =~ (^|[[:space:]])(unhealthy|unused|draining|initial|unavailable)([[:space:]]|$) ]]
}

ecs_service_is_stable() {
  local service="$1"
  local status desired running pending _task_definition
  read -r status desired running pending _task_definition < <(describe_ecs_service "$service")
  [[ "$status" == "ACTIVE" && "$desired" -gt 0 && "$running" -eq "$desired" && "$pending" -eq 0 ]]
}

rds_instance_is_available() {
  local instance="$1"
  local status _rest
  read -r status _rest < <(describe_rds_instance "$instance")
  [[ "$status" == "available" ]]
}

production_invariants_are_healthy() {
  local alb alb_arn alb_state _alb_dns listener_arn rules

  ecs_service_is_stable "$MYCARESIGNALS_PROD_STAFF_ECS_SERVICE" || return 1
  ecs_service_is_stable "$MYCARESIGNALS_PROD_FEEDBACK_ECS_SERVICE" || return 1
  rds_instance_is_available "$MYCARESIGNALS_PROD_RDS_INSTANCE" || return 1
  rds_instance_is_available "$MYCARESIGNALS_PROD_FEEDBACK_RDS_INSTANCE" || return 1
  production_target_is_healthy "$MYCARESIGNALS_PROD_STAFF_TARGET_GROUP" || return 1
  production_target_is_healthy "$MYCARESIGNALS_PROD_FEEDBACK_TARGET_GROUP" || return 1

  alb="$(shared_alb_details)" || return 1
  read -r alb_arn alb_state _alb_dns <<<"$alb"
  [[ "$alb_state" == "active" ]] || return 1
  listener_arn="$(shared_https_listener_arn "$alb_arn")" || return 1
  [[ -n "$listener_arn" && "$listener_arn" != "None" ]] || return 1
  rules="$(listener_rules_json "$listener_arn")" || return 1
  grep -Fq 'api.mycaresignals.co.uk' <<<"$rules" \
    && grep -Fq '/api/v1/feedback' <<<"$rules" \
    && grep -Fq 'api.demo.mycaresignals.co.uk' <<<"$rules"
}

capture_production_invariants() {
  local alb alb_arn _alb_state _alb_dns listener_arn
  local staff_ecs feedback_ecs staff_rds feedback_rds
  local staff_targets feedback_targets routing

  production_invariants_are_healthy || {
    guard_error "production invariants are not healthy"
    return 1
  }

  alb="$(shared_alb_details)" || return 1
  read -r alb_arn _alb_state _alb_dns <<<"$alb"
  listener_arn="$(shared_https_listener_arn "$alb_arn")" || return 1
  staff_ecs="$(describe_ecs_service "$MYCARESIGNALS_PROD_STAFF_ECS_SERVICE")" || return 1
  feedback_ecs="$(describe_ecs_service "$MYCARESIGNALS_PROD_FEEDBACK_ECS_SERVICE")" || return 1
  staff_rds="$(describe_rds_instance "$MYCARESIGNALS_PROD_RDS_INSTANCE")" || return 1
  feedback_rds="$(describe_rds_instance "$MYCARESIGNALS_PROD_FEEDBACK_RDS_INSTANCE")" || return 1
  staff_targets="$(target_health_summary "$MYCARESIGNALS_PROD_STAFF_TARGET_GROUP")" || return 1
  feedback_targets="$(target_health_summary "$MYCARESIGNALS_PROD_FEEDBACK_TARGET_GROUP")" || return 1
  routing="$(listener_rules_json "$listener_arn")" || return 1

  printf 'staff_ecs=%s\n' "$staff_ecs"
  printf 'feedback_ecs=%s\n' "$feedback_ecs"
  printf 'staff_rds=%s\n' "$staff_rds"
  printf 'feedback_rds=%s\n' "$feedback_rds"
  printf 'alb=%s\n' "$alb"
  printf 'staff_targets=%s\n' "$staff_targets"
  printf 'feedback_targets=%s\n' "$feedback_targets"
  printf 'routing=%s\n' "$routing"
}

assert_production_invariants_unchanged() {
  local before="$1"
  local after="$2"

  if [[ "$before" != "$after" ]]; then
    guard_error "production invariants changed during demo lifecycle operation"
    return 1
  fi
}
