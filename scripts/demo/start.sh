#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/lifecycle.sh
source "${SCRIPT_DIR}/lib/lifecycle.sh"

start_demo_main "$@"
