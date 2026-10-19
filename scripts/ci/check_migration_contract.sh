#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

latest_version() {
  local directory="$1"
  find "$directory" -maxdepth 1 -type f -name '*.up.sql' -printf '%f\n' |
    sed -E 's/^0*([0-9]+)_.*/\1/' |
    sort -n |
    tail -n 1
}

require_down_migration() {
  local directory="$1"
  local expected="$2"
  if ! find "$directory" -maxdepth 1 -type f -name "0*${expected}_*.down.sql" -print -quit | grep -q .; then
    echo "Latest migration ${expected} in ${directory} has no down migration." >&2
    exit 1
  fi
}

staff_dir="apps/mycaresignals/staff/internal/migrations"
feedback_dir="apps/mycaresignals/feedback/internal/migrations"
staff_latest="$(latest_version "$staff_dir")"
feedback_latest="$(latest_version "$feedback_dir")"

[[ "$staff_latest" == "32" ]] || { echo "Staff latest migration is ${staff_latest}, expected 32." >&2; exit 1; }
[[ "$feedback_latest" == "19" ]] || { echo "Feedback latest migration is ${feedback_latest}, expected 19." >&2; exit 1; }

require_down_migration "$staff_dir" "$staff_latest"
require_down_migration "$feedback_dir" "$feedback_latest"

grep -Fq 'MIGRATION_EXPECTED_VERSION", value = "32"' infrastructure/envs/mycaresignals/prod/services/teapi/main.tf
grep -Fq 'MIGRATION_EXPECTED_VERSION", value = "32"' infrastructure/envs/mycaresignals/demo/services/teapi/main.tf
grep -Fq 'MIGRATION_EXPECTED_VERSION", value = "019"' infrastructure/envs/mycaresignals/prod/services/qrfeedback/main.tf

echo "Migration contract checks passed (Staff 32, Feedback 19)."
