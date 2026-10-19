#!/bin/sh

set -eu

LOCAL_SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$LOCAL_SCRIPT_DIR/common.sh"

ensure_local_environment
require_local_scope

runtime_dir="$LOCAL_APP_DIR/.local"
context_token_file="$runtime_dir/feedback-context-token"
mkdir -p "$runtime_dir"
chmod 700 "$runtime_dir"

staff_dsn="postgres://${STAFF_DB_USER}:${STAFF_DB_PASSWORD}@127.0.0.1:${STAFF_DB_PORT:-5433}/${STAFF_DB_NAME}?sslmode=disable"
API_BASE="http://127.0.0.1:${STAFF_API_PORT:-8081}" \
POSTGRES_DSN="$staff_dsn" \
node "$LOCAL_APP_DIR/staff/scripts/seed_stakeholder_demo.mjs"

PGPASSWORD="$STAFF_DB_PASSWORD" psql \
  --host 127.0.0.1 \
  --port "${STAFF_DB_PORT:-5433}" \
  --username "$STAFF_DB_USER" \
  --dbname "$STAFF_DB_NAME" \
  --set ON_ERROR_STOP=1 \
  --quiet \
  --command "UPDATE homes SET name = 'Synthetic Millbrow House' WHERE external_ref = 'MCS-DEMO-HOME-V1'"

context_token=$(openssl rand -hex 32)
context_hash=$(CONTEXT_TOKEN="$context_token" FEEDBACK_SECRET_KEY="$FEEDBACK_SECRET_KEY" node -e '
  const { createHmac } = require("node:crypto");
  process.stdout.write(createHmac("sha256", process.env.FEEDBACK_SECRET_KEY)
    .update(`intake-context:${process.env.CONTEXT_TOKEN}`).digest("hex"));
')

PGPASSWORD="$FEEDBACK_DB_PASSWORD" psql \
  --host 127.0.0.1 \
  --port "${FEEDBACK_DB_PORT:-5434}" \
  --username "$FEEDBACK_DB_USER" \
  --dbname "$FEEDBACK_DB_NAME" \
  --set ON_ERROR_STOP=1 \
  --set token_hash="$context_hash" \
  --quiet <<'SQL'
INSERT INTO intake_contexts (
  token_hash, home_external_id, home_label, purpose, expires_at
)
VALUES (
  :'token_hash', 1, 'Synthetic Millbrow House', 'visitor_feedback', now() + interval '30 days'
);
SQL

umask 077
printf '%s' "$context_token" >"$context_token_file"

echo "Synthetic Staff fixture: Alex Demo at Synthetic Millbrow House"
echo "Synthetic Feedback context created; token stored in ignored local file: $context_token_file"
