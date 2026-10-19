#!/bin/sh

set -eu

LOCAL_SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$LOCAL_SCRIPT_DIR/common.sh"

ensure_local_environment
require_local_scope

if [ "$#" -ne 0 ]; then
  echo "reset.sh accepts no targets or environment arguments." >&2
  exit 1
fi

echo "Resetting only local synthetic data for Compose project: $LOCAL_PROJECT_NAME"
compose_local down --volumes --remove-orphans
"$LOCAL_SCRIPT_DIR/start.sh"
