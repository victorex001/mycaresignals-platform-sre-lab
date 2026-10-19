#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

foundation="infrastructure/envs/mycaresignals/prod/foundation/main.tf"
staff="infrastructure/envs/mycaresignals/prod/services/teapi/main.tf"
feedback="infrastructure/envs/mycaresignals/prod/services/qrfeedback/main.tf"

alarm_block() {
  local file="$1"
  local name="$2"
  sed -n "/resource \"aws_cloudwatch_metric_alarm\" \"${name}\"/,/^[[:space:]]*tags[[:space:]]*=/p" "$file"
}

assert_contains() {
  local content="$1"
  local pattern="$2"
  local description="$3"
  if ! grep -Fq "$pattern" <<<"$content"; then
    echo "Missing ${description}." >&2
    exit 1
  fi
}

for spec in \
  "$foundation:alb_5xx" \
  "$foundation:rds_cpu" \
  "$foundation:rds_storage" \
  "$staff:running_tasks_low" \
  "$feedback:api_running_tasks_low"; do
  file="${spec%:*}"
  name="${spec##*:}"
  block="$(alarm_block "$file" "$name")"
  assert_contains "$block" "alarm_actions" "alarm action for ${name}"
  assert_contains "$block" "ok_actions" "recovery action for ${name}"
done

assert_contains "$(alarm_block "$foundation" alb_5xx)" 'treat_missing_data  = "notBreaching"' "ALB missing-data policy"
assert_contains "$(alarm_block "$staff" running_tasks_low)" 'treat_missing_data  = "breaching"' "Staff task-count missing-data policy"
assert_contains "$(alarm_block "$feedback" api_running_tasks_low)" 'treat_missing_data  = "breaching"' "Feedback task-count missing-data policy"

grep -Fq 'resource "aws_sns_topic" "operations_alerts"' "$foundation"
grep -Fq 'kms_master_key_id = "alias/aws/sns"' "$foundation"

echo "Observability contract checks passed for five production alarms."
