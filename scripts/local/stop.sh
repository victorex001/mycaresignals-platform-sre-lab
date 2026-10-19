#!/bin/sh

set -eu

LOCAL_SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$LOCAL_SCRIPT_DIR/common.sh"

ensure_local_environment
require_local_scope
compose_local down --remove-orphans
