#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

status=0

while IFS= read -r path; do
  printf 'Forbidden generated or local configuration artifact: %s\n' "$path" >&2
  status=1
done < <(
  find . \
    -path './.git' -prune -o \
    -path '*/node_modules' -prune -o \
    -path '*/.terraform' -prune -o \
    -type f \( \
      -name 'terraform.tfvars' -o \
      -name '*.tfstate' -o \
      -name '*.tfstate.*' -o \
      -name 'tfplan' -o \
      -name '*.tfplan' -o \
      -path './apps/mycaresignals/staff/main' -o \
      -path './apps/mycaresignals/feedback/main' -o \
      -name '.env' -o \
      -name '*.pem' -o \
      -name '*.key' -o \
      -name '*.p12' -o \
      -name '*.pfx' -o \
      -name 'id_rsa' -o \
      -name 'kubeconfig' \
    \) -print
)

while IFS= read -r path; do
  printf 'High-confidence credential signature detected in: %s\n' "$path" >&2
  status=1
done < <(
  find . \
    -path './.git' -prune -o \
    -path '*/node_modules' -prune -o \
    -path '*/dist' -prune -o \
    -path '*/.terraform' -prune -o \
    -type f \
    ! -path './scripts/ci/check_repository_safety.sh' \
    ! -name '*.example' \
    ! -name 'package-lock.json' \
    -print0 |
    xargs -0 -r grep -IlE \
      '(AKIA|ASIA)[A-Z0-9]{16}|-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----|glpat-[A-Za-z0-9_-]{20,}|glrt-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9_]{30,}|npm_[A-Za-z0-9]{30,}' || true
)

if (( status != 0 )); then
  exit "$status"
fi

echo "Repository safety checks passed."
