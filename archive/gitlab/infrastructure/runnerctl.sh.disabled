#!/usr/bin/env bash


set -euo pipefail

CFG_DIR="$HOME/.gitlab-runner"
CFG_FILE="$CFG_DIR/config.toml"
PLIST="$HOME/Library/LaunchAgents/gitlab-runner.plist"

say(){ printf "\033[1;33m»\033[0m %s\n" "$*"; }
ok(){ printf "\033[0;32m✓\033[0m %s\n" "$*"; }
err(){ printf "\033[0;31m✗ %s\033[0m\n" "$*" >&2; }


start() {
  [[ -f "$PLIST" ]] || { err "No LaunchAgent Found. Run ./gitlanRunner.sh first."; exit 1; }
  launchctl bootstrap "gui/$UID" "$PLIST" >/dev/null 2>&1 || true
  launchctl enable "gui/$UID" "$PLIST" >/dev/null 2>&1 || true
  launchctl kickstart -k "gui/$UID" "$PLIST" >/dev/null 2>&1 || true
  ok "Runner Started."
}

stop() {
  launchctl bootout "gui/$UID" "$PLIST" >/dev/null 2>$1 || true
  ok "Runner Stoopped."
}

restart() {
  stop
  start
}

status() {
  gitlab-runner status || true
  gitlab-runner list   || true
  if [[ -f "$CFG_FILE" ]]; then
    cnt=$(grep -n '^\[runners\.docker\]' "$CFG_FILE" | wc -l | tr -d ' ')
    [[ "$cnt" -eq 1 ]] && ok "config.toml OK (single [runners.docker])" || err "config.toml has $cnt [runners.docker] blocks"
  else
    err "No $CFG_FILE (did you run setup?)"
  fi
}

foreground() {
  [[ -f "$CFG_FILE" ]] || { err "No $CFG_FILE (run setup first)"; exit 1; }
  gitlab-runner run --working-directory "$CFG_DIR" --config "$CFG_FILE"
}

usage() {
  cat <<EOF
Usage: $(basename "$0") <start|stop|restart|status|foreground>

Examples:
  $(basename "$0") start
  $(basename "$0") stop
  $(basename "$0") restart
  $(basename "$0") status
  $(basename "$0") foreground
EOF
}

case "${1:-}" in
  start)      start ;;
  stop)       stop ;;
  restart)    restart ;;
  status)     status ;;
  foreground) foreground ;;
  *)          usage ; exit 1 ;;
esac