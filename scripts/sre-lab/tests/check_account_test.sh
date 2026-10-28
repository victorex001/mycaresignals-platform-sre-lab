#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
check_script="$(cd "$script_dir/.." && pwd)/check-account.sh"
test_bin="$(mktemp -d)"
trap 'rm -rf "$test_bin"' EXIT
ln -s "$script_dir/fake-aws.sh" "$test_bin/aws"

passed=0

expect_failure() {
  if PATH="$test_bin:$PATH" "$@" >/dev/null 2>&1; then
    echo "Expected failure: $*" >&2
    exit 1
  fi
  passed=$((passed + 1))
}

expect_success() {
  if ! PATH="$test_bin:$PATH" "$@" >/dev/null 2>&1; then
    echo "Expected success: $*" >&2
    exit 1
  fi
  passed=$((passed + 1))
}

expect_failure env -u SRE_LAB_ACCOUNT_ID FAKE_AWS_ACCOUNT=266877689034 "$check_script"
expect_failure env SRE_LAB_ACCOUNT_ID=121638211660 FAKE_AWS_ACCOUNT=266877689034 "$check_script"
expect_failure env SRE_LAB_ACCOUNT_ID=266877689034 FAKE_AWS_ACCOUNT=121638211660 "$check_script"
expect_failure env SRE_LAB_ACCOUNT_ID=266877689034 FAKE_AWS_ACCOUNT=210987654321 "$check_script"
expect_success env SRE_LAB_ACCOUNT_ID=266877689034 FAKE_AWS_ACCOUNT=266877689034 "$check_script"

expect_failure env SRE_LAB_ACCOUNT_ID=210987654321 FAKE_AWS_ACCOUNT=210987654321 "$check_script"
expect_failure env AWS_REGION=us-east-1 SRE_LAB_ACCOUNT_ID=266877689034 FAKE_AWS_ACCOUNT=266877689034 "$check_script"
expect_failure env SRE_LAB_ACCOUNT_ID=266877689034 FAKE_AWS_ACCOUNT=not-an-account "$check_script"
echo "SRE lab account guard tests passed: ${passed}/8."
