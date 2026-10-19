#!/bin/sh

set -eu

LOCAL_SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
LOCAL_REPO_ROOT=$(CDPATH= cd -- "$LOCAL_SCRIPT_DIR/../.." && pwd)
LOCAL_APP_DIR="$LOCAL_REPO_ROOT/apps/mycaresignals"
LOCAL_COMPOSE_FILE="$LOCAL_APP_DIR/compose.local.yaml"
LOCAL_ENV_FILE="$LOCAL_APP_DIR/.env.local"
LOCAL_ENV_EXAMPLE="$LOCAL_APP_DIR/.env.local.example"
LOCAL_PROJECT_NAME="mycaresignals-local"

ensure_local_environment() {
  if [ ! -f "$LOCAL_ENV_FILE" ]; then
    install -m 600 "$LOCAL_ENV_EXAMPLE" "$LOCAL_ENV_FILE"
    echo "Created local-only configuration: $LOCAL_ENV_FILE"
  fi

  set -a
  # shellcheck disable=SC1090
  . "$LOCAL_ENV_FILE"
  set +a
}

compose_local() {
  docker-compose \
    --project-name "$LOCAL_PROJECT_NAME" \
    --env-file "$LOCAL_ENV_FILE" \
    --file "$LOCAL_COMPOSE_FILE" \
    "$@"
}

require_local_scope() {
  configured_name=$(compose_local config --format json | jq -r '.name')
  if [ "$configured_name" != "$LOCAL_PROJECT_NAME" ]; then
    echo "Refusing operation: unexpected Compose project '$configured_name'." >&2
    exit 1
  fi
  case "$LOCAL_COMPOSE_FILE" in
    "$LOCAL_REPO_ROOT"/apps/mycaresignals/compose.local.yaml) ;;
    *)
      echo "Refusing operation: Compose file is outside the local monorepo path." >&2
      exit 1
      ;;
  esac
}

wait_for_url() {
  label=$1
  url=$2
  host_header=${3:-}
  attempts=0
  while [ "$attempts" -lt 120 ]; do
    if [ -n "$host_header" ]; then
      if curl --fail --silent --show-error --header "Host: $host_header" --output /dev/null "$url" 2>/dev/null; then
        echo "$label: ready"
        return 0
      fi
    elif curl --fail --silent --show-error --output /dev/null "$url" 2>/dev/null; then
      echo "$label: ready"
      return 0
    fi
    attempts=$((attempts + 1))
    sleep 1
  done
  echo "$label did not become ready: $url" >&2
  return 1
}
