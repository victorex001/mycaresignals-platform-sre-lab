#!/usr/bin/env bash

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIFECYCLE_LIB="$(cd "${TEST_DIR}/../lib" && pwd)/lifecycle.sh"
TEST_TMP="$(mktemp -d)"
trap 'rm -rf "$TEST_TMP"' EXIT

passed=0
failed=0

pass() { printf 'ok - %s\n' "$1"; passed=$((passed + 1)); }
fail() { printf 'not ok - %s\n' "$1"; failed=$((failed + 1)); }

check_case() {
  local name="$1"
  local function_name="$2"
  if "$function_name"; then pass "$name"; else fail "$name"; fi
}

mock_common() {
  require_demo_guards() { :; }
  acquire_demo_lifecycle_lock() { :; }
  release_demo_lifecycle_lock() { :; }
  install_lifecycle_traps() { :; }
  capture_production_invariants() { printf '%s\n' "${INVARIANT_VALUE:-stable}"; }
  assert_production_invariants_unchanged() { [[ "$1" == "$2" ]]; }
  run_demo_verify() { printf 'verify\n' >>"$EVENT_LOG"; }
  production_invariants_are_healthy() { :; }
  wait_for_demo_ecs_stable() { :; }
}

case_start_already_on() (
  # shellcheck source=../lib/lifecycle.sh
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/start-on.log"; : >"$EVENT_LOG"
  mock_common
  describe_rds_instance() { printf 'available True db.t4g.micro 20\n'; }
  describe_ecs_service() { printf 'ACTIVE 1 1 0 task-definition\n'; }
  wait_for_rds_state() { printf 'wait-rds\n' >>"$EVENT_LOG"; }
  set_demo_ecs_desired_count() { printf 'unexpected-ecs-mutation\n' >>"$EVENT_LOG"; return 1; }
  start_demo_rds() { printf 'unexpected-rds-mutation\n' >>"$EVENT_LOG"; return 1; }
  wait_for_demo_ecs_counts() { printf 'wait-ecs\n' >>"$EVENT_LOG"; }
  wait_for_demo_target_healthy() { printf 'target-healthy\n' >>"$EVENT_LOG"; }
  start_demo_main >"${TEST_TMP}/start-on.out"
  ! grep -Fq 'unexpected' "$EVENT_LOG" \
    && grep -Fq 'MYCARESIGNALS DEMO STAFF: ON' "${TEST_TMP}/start-on.out"
)

case_start_ordering() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/start-order.log"; : >"$EVENT_LOG"
  mock_common
  describe_rds_instance() { printf 'stopped True db.t4g.micro 20\n'; }
  describe_ecs_service() { printf 'ACTIVE 0 0 0 task-definition\n'; }
  start_demo_rds() { printf 'rds-start\n' >>"$EVENT_LOG"; }
  wait_for_rds_state() { printf 'rds-ready\n' >>"$EVENT_LOG"; }
  set_demo_ecs_desired_count() { printf 'ecs-one\n' >>"$EVENT_LOG"; }
  wait_for_demo_ecs_stable() { printf 'ecs-stable\n' >>"$EVENT_LOG"; }
  wait_for_demo_ecs_counts() { printf 'ecs-ready\n' >>"$EVENT_LOG"; }
  wait_for_demo_target_healthy() { printf 'target-healthy\n' >>"$EVENT_LOG"; }
  start_demo_main >/dev/null
  [[ "$(tr '\n' ' ' <"$EVENT_LOG")" == 'rds-start rds-ready ecs-one ecs-stable ecs-ready target-healthy verify ' ]]
)

case_start_starting_and_ecs_scaling() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/start-scaling.log"; : >"$EVENT_LOG"
  mock_common
  describe_rds_instance() { printf 'starting True db.t4g.micro 20\n'; }
  describe_ecs_service() { printf 'ACTIVE 1 0 1 task-definition\n'; }
  start_demo_rds() { printf 'unexpected-rds-mutation\n' >>"$EVENT_LOG"; return 1; }
  set_demo_ecs_desired_count() { printf 'unexpected-ecs-mutation\n' >>"$EVENT_LOG"; return 1; }
  wait_for_rds_state() { :; }
  wait_for_demo_ecs_counts() { :; }
  wait_for_demo_target_healthy() { :; }
  start_demo_main >/dev/null
  ! grep -Fq 'unexpected' "$EVENT_LOG"
)

case_stop_already_off() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/stop-off.log"; : >"$EVENT_LOG"
  mock_common
  describe_ecs_service() { printf 'ACTIVE 0 0 0 task-definition\n'; }
  describe_rds_instance() { printf 'stopped True db.t4g.micro 20\n'; }
  set_demo_ecs_desired_count() { printf 'unexpected-ecs-mutation\n' >>"$EVENT_LOG"; return 1; }
  stop_demo_rds() { printf 'unexpected-rds-mutation\n' >>"$EVENT_LOG"; return 1; }
  wait_for_demo_ecs_counts() { :; }
  wait_for_demo_target_drained() { :; }
  assert_no_demo_migration_task_running() { :; }
  wait_for_rds_state() { :; }
  stop_demo_main >"${TEST_TMP}/stop-off.out"
  ! grep -Fq 'unexpected' "$EVENT_LOG" \
    && grep -Fq 'MYCARESIGNALS DEMO STAFF: OFF' "${TEST_TMP}/stop-off.out"
)

case_stop_ordering_and_warning() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/stop-order.log"; : >"$EVENT_LOG"
  mock_common
  describe_ecs_service() { printf 'ACTIVE 1 1 0 task-definition\n'; }
  describe_rds_instance() { printf 'available True db.t4g.micro 20\n'; }
  set_demo_ecs_desired_count() { printf 'ecs-zero\n' >>"$EVENT_LOG"; }
  wait_for_demo_ecs_stable() { printf 'ecs-stable\n' >>"$EVENT_LOG"; }
  wait_for_demo_ecs_counts() { printf 'ecs-zero-ready\n' >>"$EVENT_LOG"; }
  wait_for_demo_target_drained() { printf 'target-drained\n' >>"$EVENT_LOG"; }
  assert_no_demo_migration_task_running() { printf 'migration-clear\n' >>"$EVENT_LOG"; }
  stop_demo_rds() { printf 'rds-stop\n' >>"$EVENT_LOG"; }
  wait_for_rds_state() { printf 'rds-stopped\n' >>"$EVENT_LOG"; }
  stop_demo_main >"${TEST_TMP}/stop-order.out"
  [[ "$(tr '\n' ' ' <"$EVENT_LOG")" == 'ecs-zero ecs-stable ecs-zero-ready target-drained migration-clear rds-stop rds-stopped ' ]] \
    && grep -Fq 'seven' <(tr '[:upper:]' '[:lower:]' <"${TEST_TMP}/stop-order.out") \
    && grep -Fq 'automatically restarts' "${TEST_TMP}/stop-order.out"
)

case_stop_rds_stopping() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/stop-stopping.log"; : >"$EVENT_LOG"
  mock_common
  describe_ecs_service() { printf 'ACTIVE 0 0 0 task-definition\n'; }
  describe_rds_instance() { printf 'stopping True db.t4g.micro 20\n'; }
  wait_for_demo_ecs_counts() { :; }
  wait_for_demo_target_drained() { :; }
  assert_no_demo_migration_task_running() { :; }
  stop_demo_rds() { printf 'unexpected-rds-mutation\n' >>"$EVENT_LOG"; return 1; }
  wait_for_rds_state() { :; }
  stop_demo_main >/dev/null
  ! grep -Fq 'unexpected' "$EVENT_LOG"
)

case_migration_blocks_rds_stop() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/migration.log"; : >"$EVENT_LOG"
  mock_common
  describe_ecs_service() { printf 'ACTIVE 1 1 0 task-definition\n'; }
  describe_rds_instance() { printf 'available True db.t4g.micro 20\n'; }
  set_demo_ecs_desired_count() { printf 'ecs-zero\n' >>"$EVENT_LOG"; }
  wait_for_demo_ecs_counts() { :; }
  wait_for_demo_target_drained() { :; }
  assert_no_demo_migration_task_running() { return 1; }
  stop_demo_rds() { printf 'rds-stop\n' >>"$EVENT_LOG"; }
  if stop_demo_main >/dev/null 2>&1; then return 1; fi
  ! grep -Fq 'rds-stop' "$EVENT_LOG"
)

case_invariant_failure_blocks_mutation() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/invariant.log"; : >"$EVENT_LOG"
  mock_common
  capture_production_invariants() { return 1; }
  describe_rds_instance() { printf 'stopped True db.t4g.micro 20\n'; }
  start_demo_rds() { printf 'mutation\n' >>"$EVENT_LOG"; }
  if start_demo_main >/dev/null 2>&1; then return 1; fi
  [[ ! -s "$EVENT_LOG" ]]
)

case_invariant_change_detected() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/invariant-change.log"; : >"$EVENT_LOG"
  mock_common
  capture_count_file="${TEST_TMP}/capture-count"
  capture_production_invariants() {
    local capture_count=0
    [[ -f "$capture_count_file" ]] && capture_count="$(<"$capture_count_file")"
    capture_count=$((capture_count + 1))
    printf '%s\n' "$capture_count" >"$capture_count_file"
    printf 'snapshot-%s\n' "$capture_count"
  }
  describe_rds_instance() { printf 'available True db.t4g.micro 20\n'; }
  describe_ecs_service() { printf 'ACTIVE 1 1 0 task-definition\n'; }
  wait_for_rds_state() { :; }
  wait_for_demo_ecs_counts() { :; }
  wait_for_demo_target_healthy() { :; }
  if start_demo_main >/dev/null 2>&1; then return 1; fi
)

case_lock_contention_blocks() (
  source "$LIFECYCLE_LIB"
  EVENT_LOG="${TEST_TMP}/lock.log"; : >"$EVENT_LOG"
  mock_common
  acquire_demo_lifecycle_lock() { return 1; }
  describe_rds_instance() { printf 'stopped True db.t4g.micro 20\n'; }
  start_demo_rds() { printf 'mutation\n' >>"$EVENT_LOG"; }
  if start_demo_main >/dev/null 2>&1; then return 1; fi
  [[ ! -s "$EVENT_LOG" ]]
)

case_real_lock_contention() (
  export TMPDIR="${TEST_TMP}/lock-root"
  mkdir -p "$TMPDIR"
  source "$LIFECYCLE_LIB"
  acquire_demo_lifecycle_lock "first"
  if TMPDIR="$TMPDIR" bash -c "source '${LIFECYCLE_LIB}'; acquire_demo_lifecycle_lock second" >/dev/null 2>&1; then
    release_demo_lifecycle_lock
    return 1
  fi
  release_demo_lifecycle_lock
)

case_exact_mutation_commands() (
  mock_aws="${TEST_TMP}/mock-aws"
  aws_log="${TEST_TMP}/aws.log"
  cat >"$mock_aws" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$MOCK_AWS_LOG"
printf 'ok\n'
MOCK
  chmod +x "$mock_aws"
  export AWS_CLI="$mock_aws" MOCK_AWS_LOG="$aws_log"
  source "$LIFECYCLE_LIB"
  start_demo_rds
  stop_demo_rds
  set_demo_ecs_desired_count 1
  grep -Fq 'rds start-db-instance --db-instance-identifier mycaresignals-demo-pg' "$aws_log" \
    && grep -Fq 'rds stop-db-instance --db-instance-identifier mycaresignals-demo-pg' "$aws_log" \
    && grep -Fq 'ecs update-service --cluster mycaresignals-prod-cluster --service mycaresignals-demo-teapi --desired-count 1' "$aws_log" \
    && [[ "$(wc -l <"$aws_log" | tr -d ' ')" == "3" ]]
)

case_mutation_target_guard_blocks_aws() (
  mock_aws="${TEST_TMP}/guard-mock-aws"
  aws_log="${TEST_TMP}/guard-aws.log"
  cat >"$mock_aws" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$MOCK_AWS_LOG"
MOCK
  chmod +x "$mock_aws"
  export AWS_CLI="$mock_aws" MOCK_AWS_LOG="$aws_log"
  source "$LIFECYCLE_LIB"
  assert_safe_demo_mutation_target() { return 1; }
  if start_demo_rds >/dev/null 2>&1; then return 1; fi
  if stop_demo_rds >/dev/null 2>&1; then return 1; fi
  [[ ! -e "$aws_log" ]]
)

case_shared_parent_guard_blocks_aws() (
  mock_aws="${TEST_TMP}/parent-mock-aws"
  aws_log="${TEST_TMP}/parent-aws.log"
  cat >"$mock_aws" <<'MOCK'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$MOCK_AWS_LOG"
MOCK
  chmod +x "$mock_aws"
  export AWS_CLI="$mock_aws" MOCK_AWS_LOG="$aws_log"
  source "$LIFECYCLE_LIB"
  assert_safe_demo_mutation_target() { :; }
  assert_read_only_shared_parent() { return 1; }
  if set_demo_ecs_desired_count 1 >/dev/null 2>&1; then return 1; fi
  [[ ! -e "$aws_log" ]]
)

check_case "start already-on is idempotent" case_start_already_on
check_case "start orders RDS before ECS" case_start_ordering
check_case "start waits through RDS/ECS scaling" case_start_starting_and_ecs_scaling
check_case "stop already-off is idempotent" case_stop_already_off
check_case "stop orders ECS before RDS and warns" case_stop_ordering_and_warning
check_case "stop waits through RDS stopping" case_stop_rds_stopping
check_case "migration task blocks RDS stop" case_migration_blocks_rds_stop
check_case "unhealthy production invariant blocks mutation" case_invariant_failure_blocks_mutation
check_case "changed production invariant is detected" case_invariant_change_detected
check_case "lifecycle lock contention blocks operation" case_lock_contention_blocks
check_case "shared lifecycle lock rejects a second holder" case_real_lock_contention
check_case "mutation commands use exact allowlisted targets" case_exact_mutation_commands
check_case "failed mutation-target guard blocks AWS" case_mutation_target_guard_blocks_aws
check_case "failed shared-parent guard blocks AWS" case_shared_parent_guard_blocks_aws

printf '\n%s passed, %s failed\n' "$passed" "$failed"
[[ "$failed" -eq 0 ]]
