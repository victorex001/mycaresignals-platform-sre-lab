#!/usr/bin/env bash

set -euo pipefail

DEMO_LIFECYCLE_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=status.sh
source "${DEMO_LIFECYCLE_LIB_DIR}/status.sh"

readonly DEMO_WAIT_ATTEMPTS="${DEMO_WAIT_ATTEMPTS:-90}"
readonly DEMO_WAIT_INTERVAL_SECONDS="${DEMO_WAIT_INTERVAL_SECONDS:-10}"

aws_demo_mutation() {
  "$MYCARESIGNALS_AWS_CLI" --no-cli-pager --region "$MYCARESIGNALS_EXPECTED_REGION" "$@"
}

start_demo_rds() {
  assert_safe_demo_mutation_target "$MYCARESIGNALS_DEMO_RDS_INSTANCE" || return 1
  log_lifecycle_event "start" "$MYCARESIGNALS_DEMO_RDS_INSTANCE" "requested" "starting exact demo RDS instance"
  aws_demo_mutation rds start-db-instance \
    --db-instance-identifier "$MYCARESIGNALS_DEMO_RDS_INSTANCE" \
    --query 'DBInstance.DBInstanceIdentifier' --output text >/dev/null
}

stop_demo_rds() {
  assert_safe_demo_mutation_target "$MYCARESIGNALS_DEMO_RDS_INSTANCE" || return 1
  log_lifecycle_event "stop" "$MYCARESIGNALS_DEMO_RDS_INSTANCE" "requested" "stopping exact demo RDS instance"
  aws_demo_mutation rds stop-db-instance \
    --db-instance-identifier "$MYCARESIGNALS_DEMO_RDS_INSTANCE" \
    --query 'DBInstance.DBInstanceIdentifier' --output text >/dev/null
}

set_demo_ecs_desired_count() {
  local desired_count="$1"
  [[ "$desired_count" == "0" || "$desired_count" == "1" ]] || {
    guard_error "demo ECS desired count must be exactly 0 or 1"
    return 1
  }
  assert_safe_demo_mutation_target "$MYCARESIGNALS_DEMO_ECS_SERVICE" || return 1
  assert_read_only_shared_parent "$MYCARESIGNALS_SHARED_ECS_CLUSTER" || return 1
  log_lifecycle_event "ecs-scale" "$MYCARESIGNALS_DEMO_ECS_SERVICE" "requested" "desired_count=${desired_count}"
  aws_demo_mutation ecs update-service \
    --cluster "$MYCARESIGNALS_SHARED_ECS_CLUSTER" \
    --service "$MYCARESIGNALS_DEMO_ECS_SERVICE" \
    --desired-count "$desired_count" \
    --query 'service.serviceName' --output text >/dev/null
}

wait_for_rds_state() {
  local expected_state="$1"
  local attempt details state _rest

  for ((attempt = 1; attempt <= DEMO_WAIT_ATTEMPTS; attempt += 1)); do
    details="$(describe_rds_instance "$MYCARESIGNALS_DEMO_RDS_INSTANCE")" || return 1
    read -r state _rest <<<"$details"
    [[ "$state" == "$expected_state" ]] && return 0
    sleep "$DEMO_WAIT_INTERVAL_SECONDS"
  done
  guard_error "timed out waiting for ${MYCARESIGNALS_DEMO_RDS_INSTANCE} state ${expected_state}"
  return 1
}

wait_for_demo_ecs_counts() {
  local expected_desired="$1"
  local expected_running="$2"
  local attempt details status desired running pending _task_definition

  for ((attempt = 1; attempt <= DEMO_WAIT_ATTEMPTS; attempt += 1)); do
    details="$(describe_ecs_service "$MYCARESIGNALS_DEMO_ECS_SERVICE")" || return 1
    read -r status desired running pending _task_definition <<<"$details"
    if [[ "$status" == "ACTIVE" && "$desired" -eq "$expected_desired" \
      && "$running" -eq "$expected_running" && "$pending" -eq 0 ]]; then
      return 0
    fi
    sleep "$DEMO_WAIT_INTERVAL_SECONDS"
  done
  guard_error "timed out waiting for demo ECS counts ${expected_desired}/${expected_running}/0"
  return 1
}

wait_for_demo_ecs_stable() {
  aws_readonly ecs wait services-stable \
    --cluster "$MYCARESIGNALS_SHARED_ECS_CLUSTER" \
    --services "$MYCARESIGNALS_DEMO_ECS_SERVICE"
}

wait_for_demo_target_healthy() {
  local attempt health
  for ((attempt = 1; attempt <= DEMO_WAIT_ATTEMPTS; attempt += 1)); do
    health="$(target_health_summary "$MYCARESIGNALS_DEMO_TARGET_GROUP")" || return 1
    if [[ "$health" =~ (^|[[:space:]])healthy([[:space:]]|$) \
      && ! "$health" =~ (^|[[:space:]])(unhealthy|unused|draining|initial|unavailable)([[:space:]]|$) ]]; then
      return 0
    fi
    sleep "$DEMO_WAIT_INTERVAL_SECONDS"
  done
  guard_error "timed out waiting for healthy demo target"
  return 1
}

wait_for_demo_target_drained() {
  local attempt health
  for ((attempt = 1; attempt <= DEMO_WAIT_ATTEMPTS; attempt += 1)); do
    health="$(target_health_summary "$MYCARESIGNALS_DEMO_TARGET_GROUP")" || return 1
    if [[ -z "$health" || "$health" == "None" ]]; then
      return 0
    fi
    sleep "$DEMO_WAIT_INTERVAL_SECONDS"
  done
  guard_error "timed out waiting for demo target group to drain"
  return 1
}

assert_no_demo_migration_task_running() {
  local running_tasks
  running_tasks="$(aws_readonly ecs list-tasks \
    --cluster "$MYCARESIGNALS_SHARED_ECS_CLUSTER" \
    --family "$MYCARESIGNALS_DEMO_MIGRATION_FAMILY" \
    --desired-status RUNNING \
    --query 'taskArns' --output text)" || return 1
  if [[ -n "$running_tasks" && "$running_tasks" != "None" ]]; then
    guard_error "demo migration task is running; refusing to stop RDS"
    return 1
  fi
}

run_demo_verify() {
  "${DEMO_LIFECYCLE_LIB_DIR}/../verify.sh"
}

report_partial_demo_state() {
  local rds ecs targets
  rds="$(describe_rds_instance "$MYCARESIGNALS_DEMO_RDS_INSTANCE" 2>/dev/null || printf 'unavailable')"
  ecs="$(describe_ecs_service "$MYCARESIGNALS_DEMO_ECS_SERVICE" 2>/dev/null || printf 'unavailable')"
  targets="$(target_health_summary "$MYCARESIGNALS_DEMO_TARGET_GROUP" 2>/dev/null || printf 'unavailable')"
  printf 'Partial-state report:\n  RDS: %s\n  ECS: %s\n  Target health: %s\n' "$rds" "$ecs" "$targets" >&2
  if production_invariants_are_healthy; then
    printf '  Production invariants: healthy\n' >&2
  else
    printf '  Production invariants: UNHEALTHY\n' >&2
  fi
}

lifecycle_exit() {
  local exit_code="$?"
  trap - EXIT
  if [[ "$exit_code" -ne 0 ]]; then
    report_partial_demo_state || true
  fi
  release_demo_lifecycle_lock
  exit "$exit_code"
}

install_lifecycle_traps() {
  trap lifecycle_exit EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
}

start_demo_main() {
  local production_before production_after
  local rds_details ecs_details
  local rds_state _rds_rest ecs_status desired running pending _task_definition

  require_demo_guards || return 1
  assert_safe_demo_mutation_target "$MYCARESIGNALS_DEMO_RDS_INSTANCE" || return 1
  assert_safe_demo_mutation_target "$MYCARESIGNALS_DEMO_ECS_SERVICE" || return 1
  acquire_demo_lifecycle_lock "start" || return 1
  install_lifecycle_traps
  production_before="$(capture_production_invariants)" || return 1

  rds_details="$(describe_rds_instance "$MYCARESIGNALS_DEMO_RDS_INSTANCE")" || return 1
  read -r rds_state _rds_rest <<<"$rds_details"
  case "$rds_state" in
    stopped) start_demo_rds || return 1 ;;
    starting|available) ;;
    *) guard_error "unexpected demo RDS state for start: ${rds_state}"; return 1 ;;
  esac
  wait_for_rds_state "available" || return 1

  ecs_details="$(describe_ecs_service "$MYCARESIGNALS_DEMO_ECS_SERVICE")" || return 1
  read -r ecs_status desired running pending _task_definition <<<"$ecs_details"
  [[ "$ecs_status" == "ACTIVE" ]] || {
    guard_error "unexpected demo ECS service status: ${ecs_status}"
    return 1
  }
  case "$desired" in
    0)
      [[ "$running" -eq 0 && "$pending" -eq 0 ]] || {
        guard_error "unexpected demo ECS counts while desired count is zero"
        return 1
      }
      set_demo_ecs_desired_count 1 || return 1
      ;;
    1) ;;
    *) guard_error "unexpected demo ECS desired count for start: ${desired}"; return 1 ;;
  esac
  wait_for_demo_ecs_stable || return 1
  wait_for_demo_ecs_counts 1 1 || return 1
  wait_for_demo_target_healthy || return 1
  run_demo_verify || return 1

  production_after="$(capture_production_invariants)" || return 1
  assert_production_invariants_unchanged "$production_before" "$production_after" || return 1
  printf 'MYCARESIGNALS DEMO STAFF: ON\n'
}

stop_demo_main() {
  local production_before production_after stopped_at restart_deadline
  local rds_details ecs_details
  local rds_state _rds_rest ecs_status desired _running _pending _task_definition

  require_demo_guards || return 1
  assert_safe_demo_mutation_target "$MYCARESIGNALS_DEMO_RDS_INSTANCE" || return 1
  assert_safe_demo_mutation_target "$MYCARESIGNALS_DEMO_ECS_SERVICE" || return 1
  acquire_demo_lifecycle_lock "stop" || return 1
  install_lifecycle_traps
  production_before="$(capture_production_invariants)" || return 1

  ecs_details="$(describe_ecs_service "$MYCARESIGNALS_DEMO_ECS_SERVICE")" || return 1
  read -r ecs_status desired _running _pending _task_definition <<<"$ecs_details"
  [[ "$ecs_status" == "ACTIVE" ]] || {
    guard_error "unexpected demo ECS service status: ${ecs_status}"
    return 1
  }
  case "$desired" in
    0) ;;
    1) set_demo_ecs_desired_count 0 || return 1 ;;
    *) guard_error "unexpected demo ECS desired count for stop: ${desired}"; return 1 ;;
  esac
  wait_for_demo_ecs_stable || return 1
  wait_for_demo_ecs_counts 0 0 || return 1
  wait_for_demo_target_drained || return 1
  assert_no_demo_migration_task_running || return 1

  rds_details="$(describe_rds_instance "$MYCARESIGNALS_DEMO_RDS_INSTANCE")" || return 1
  read -r rds_state _rds_rest <<<"$rds_details"
  case "$rds_state" in
    available) stop_demo_rds || return 1 ;;
    stopping|stopped) ;;
    *) guard_error "unexpected demo RDS state for stop: ${rds_state}"; return 1 ;;
  esac
  wait_for_rds_state "stopped" || return 1

  stopped_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  restart_deadline="$(date -u -v+7d +'%Y-%m-%dT%H:%M:%SZ' 2>/dev/null \
    || date -u -d '+7 days' +'%Y-%m-%dT%H:%M:%SZ')"
  printf 'RDS stopped at: %s\n' "$stopped_at"
  printf 'Approximate automatic restart deadline: %s\n' "$restart_deadline"
  printf 'WARNING: AWS automatically restarts a stopped RDS instance after the allowed seven-day stop period.\n'

  production_after="$(capture_production_invariants)" || return 1
  assert_production_invariants_unchanged "$production_before" "$production_after" || return 1
  printf 'MYCARESIGNALS DEMO STAFF: OFF\n'
}
