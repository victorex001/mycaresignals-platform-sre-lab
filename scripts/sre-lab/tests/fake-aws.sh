#!/usr/bin/env sh
set -eu

if [ "${1:-}" = "sts" ] && [ "${2:-}" = "get-caller-identity" ]; then
  printf '%s\n' "${FAKE_AWS_ACCOUNT:-}"
  exit 0
fi

exit 2
