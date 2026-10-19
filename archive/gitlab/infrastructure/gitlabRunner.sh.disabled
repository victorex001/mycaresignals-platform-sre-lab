#!/usr/bin/env bash
set -euo pipefail

# ----- Non-secret defaults -----
GITLAB_URL="https://gitlab.com"
REG_TOKEN="${GITLAB_RUNNER_REGISTRATION_TOKEN:-}"
RUNNER_DESC="Docker Infrastructure Runner - MacBook M1"
DEFAULT_IMAGE="alpine:3.20"

# ----- Paths -----
CFG_DIR="$HOME/.gitlab-runner"
CFG_FILE="$CFG_DIR/config.toml"
LAUNCH_DIR="$HOME/Library/LaunchAgents"
PLIST="$LAUNCH_DIR/gitlab-runner.plist"

say(){ printf "\033[1;33m»\033[0m %s\n" "$*"; }
ok(){  printf "\033[0;32m✓\033[0m %s\n" "$*"; }
err(){ printf "\033[0;31m✗ %s\033[0m\n" "$*" >&2; }

need(){ command -v "$1" >/dev/null 2>&1 || { err "Missing dependency: $1"; exit 1; }; }

ensure_docker_running() {
  if ! docker info >/dev/null 2>&1; then
    say "Starting Docker Desktop and waiting up to 120s..."
    open -a Docker || true
    for _ in {1..120}; do docker info >/dev/null 2>&1 && break; sleep 1; done
    docker info >/dev/null 2>&1 || { err "Docker Desktop failed to start"; exit 1; }
  fi
  ok "Docker engine is running."
}

stop_existing() {
  launchctl bootout "gui/$UID" "$PLIST" >/dev/null 2>&1 || true
  rm -f "$PLIST" >/dev/null 2>&1 || true
}

clean_old_config() {
  rm -rf "$CFG_DIR" >/dev/null 2>&1 || true
}

register_runner() {
  [[ -n "$REG_TOKEN" ]] || { err "Set GITLAB_RUNNER_REGISTRATION_TOKEN in your shell; do not commit runner tokens."; exit 1; }
  mkdir -p "$CFG_DIR"
  say "Registering runner (Docker executor + docker.sock)..."
  gitlab-runner register --non-interactive \
    --url "$GITLAB_URL" \
    --registration-token "$REG_TOKEN" \
    --executor "docker" \
    --docker-image "$DEFAULT_IMAGE" \
    --docker-volumes "/var/run/docker.sock:/var/run/docker.sock" \
    --description "$RUNNER_DESC"

  [[ -f "$CFG_FILE" ]] || { err "Registration said OK, but $CFG_FILE not found."; exit 1; }
  ok "Runner registered."
}

install_and_start() {
  mkdir -p "$LAUNCH_DIR"
  gitlab-runner install \
    --working-directory "$CFG_DIR" \
    --config "$CFG_FILE"

  launchctl bootstrap "gui/$UID" "$PLIST" >/dev/null 2>&1 || true
  launchctl enable   "gui/$UID" "$PLIST" >/devnull 2>&1 || true
  launchctl kickstart -k "gui/$UID" "$PLIST" >/dev/null 2>&1 || true
  ok "Runner service installed & started."
}

verify() {
  say "Verifying status…"
  gitlab-runner status || true
  gitlab-runner list   || true

  cnt=$(grep -n '^\[runners\.docker\]' "$CFG_FILE" | wc -l | tr -d ' ')
  if [[ "$cnt" -gt 1 ]]; then
    err "config.toml has $cnt [runners.docker] blocks (unexpected). Re-run this script."
  else
    ok "config.toml has a single [runners.docker] block."
  fi

  cat <<MSG

Next steps:
  1) In GitLab → Project → Settings → CI/CD → Runners → Edit your runner
     - Set Tags to: docker  (and mac-local if you want)
  2) In your .gitlab-ci.yml jobs add:
       tags: ["docker"]

If a job is 'pending', check:
  - This Mac is awake with Docker Desktop running
  - The job includes tags: ["docker"]
MSG
}

main() {
  [[ "$OSTYPE" == darwin* ]] || { err "macOS only"; exit 1; }
  need docker
  need gitlab-runner

  ensure_docker_running
  say "Stopping any existing runner/agent..."; stop_existing
  say "Cleaning old runner config..."; clean_old_config
  register_runner
  install_and_start
  verify
}
main "$@"
