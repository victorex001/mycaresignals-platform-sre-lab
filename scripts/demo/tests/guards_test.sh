#!/usr/bin/env bash

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="$(cd "${TEST_DIR}/.." && pwd)"

passed=0
failed=0

pass() {
  printf 'ok - %s\n' "$1"
  passed=$((passed + 1))
}

fail() {
  printf 'not ok - %s\n' "$1"
  failed=$((failed + 1))
}

run_case() {
  local name="$1"
  local expected="$2"
  local body="$3"
  if bash -c "$body" >/dev/null 2>&1; then
    actual="success"
  else
    actual="failure"
  fi
  if [[ "$actual" == "$expected" ]]; then
    pass "$name"
  else
    fail "$name (expected ${expected}, received ${actual})"
  fi
}

mock_dir="$(mktemp -d)"
trap 'rm -rf "$mock_dir"' EXIT

cat >"${mock_dir}/aws-ok" <<'MOCK'
#!/usr/bin/env bash
printf '121638211660\n'
MOCK
cat >"${mock_dir}/aws-wrong-account" <<'MOCK'
#!/usr/bin/env bash
printf '000000000000\n'
MOCK
cat >"${mock_dir}/aws-expired" <<'MOCK'
#!/usr/bin/env bash
exit 255
MOCK
chmod +x "${mock_dir}/aws-ok" "${mock_dir}/aws-wrong-account" "${mock_dir}/aws-expired"

guards="${DEMO_DIR}/lib/guards.sh"

run_case "wrong AWS account rejected" failure \
  "export MYCARESIGNALS_ENV=demo AWS_REGION=eu-west-2 AWS_CLI='${mock_dir}/aws-wrong-account'; source '${guards}'; require_demo_guards"
run_case "wrong region rejected" failure \
  "export MYCARESIGNALS_ENV=demo AWS_REGION=us-east-1 AWS_CLI='${mock_dir}/aws-ok'; source '${guards}'; require_demo_guards"
run_case "missing demo environment rejected" failure \
  "unset MYCARESIGNALS_ENV; export AWS_REGION=eu-west-2 AWS_CLI='${mock_dir}/aws-ok'; source '${guards}'; require_demo_environment"
run_case "production environment rejected" failure \
  "export MYCARESIGNALS_ENV=prod AWS_REGION=eu-west-2 AWS_CLI='${mock_dir}/aws-ok'; source '${guards}'; require_demo_environment"
run_case "production mutation target rejected" failure \
  "source '${guards}'; assert_safe_demo_mutation_target mycaresignals-prod-teapi"
run_case "production CloudFront mutation target rejected" failure \
  "source '${guards}'; assert_safe_demo_mutation_target E3GRNC26HEEY2O"
run_case "production DNS mutation target rejected" failure \
  "source '${guards}'; assert_safe_demo_mutation_target api.mycaresignals.co.uk"
run_case "shared ECR mutation target rejected" failure \
  "source '${guards}'; assert_safe_demo_mutation_target therapeutic-engine"
run_case "demo ECS allowlist accepted" success \
  "source '${guards}'; assert_safe_demo_mutation_target mycaresignals-demo-teapi"
run_case "demo RDS allowlist accepted" success \
  "source '${guards}'; assert_safe_demo_mutation_target mycaresignals-demo-pg"
run_case "prefix-only demo resource rejected" failure \
  "source '${guards}'; assert_safe_demo_mutation_target mycaresignals-demo-unreviewed"
run_case "shared ALB mutation rejected" failure \
  "source '${guards}'; assert_safe_demo_mutation_target mycaresignals-prod-alb"
run_case "shared ECS cluster mutation rejected" failure \
  "source '${guards}'; assert_safe_demo_mutation_target mycaresignals-prod-cluster"
run_case "missing or expired AWS identity fails closed" failure \
  "export MYCARESIGNALS_ENV=demo AWS_REGION=eu-west-2 AWS_CLI='${mock_dir}/aws-expired'; source '${guards}'; require_demo_guards"
run_case "valid explicit guards accepted" success \
  "export MYCARESIGNALS_ENV=demo AWS_REGION=eu-west-2 AWS_CLI='${mock_dir}/aws-ok'; source '${guards}'; require_demo_guards"
run_case "guard bundle fails closed when status is inspected" failure \
  "unset MYCARESIGNALS_ENV; export AWS_REGION=eu-west-2 AWS_CLI='${mock_dir}/aws-ok'; source '${guards}'; if require_demo_guards; then exit 0; else exit 1; fi"

printf '\n%s passed, %s failed\n' "$passed" "$failed"
[[ "$failed" -eq 0 ]]
