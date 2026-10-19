#!/bin/sh

set -eu

LOCAL_SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$LOCAL_SCRIPT_DIR/common.sh"

ensure_local_environment
require_local_scope
docker info >/dev/null

compose_local up --build --detach

wait_for_url "Staff API" "http://127.0.0.1:${STAFF_API_PORT:-8081}/healthz"
wait_for_url "Staff Web" "http://127.0.0.1:${STAFF_WEB_PORT:-5174}/"
wait_for_url "Feedback API" "http://127.0.0.1:${FEEDBACK_API_PORT:-8080}/readyz"
wait_for_url \
  "Feedback Web" \
  "http://127.0.0.1:${FEEDBACK_WEB_PORT:-5173}/" \
  "visitorfeedback.localhost:${FEEDBACK_WEB_PORT:-5173}"

compose_local ps --all
