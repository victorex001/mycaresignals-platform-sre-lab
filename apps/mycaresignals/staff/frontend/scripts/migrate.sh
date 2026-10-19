#!/bin/sh
set -eux

echo "Listing /migrations..."
ls -la /migrations

count=0
for f in /migrations/*.sql; do
  [ -f "$f" ] || continue
  count=$((count+1))
  echo "==> Running $f"
  : "${POSTGRES_DSN:?Set POSTGRES_DSN before running frontend/scripts/migrate.sh}"
  psql "$POSTGRES_DSN" \
    -v ON_ERROR_STOP=1 \
    -f "$f"
done

[ "$count" -gt 0 ] || (echo "ERROR: No .sql files found in /migrations" && exit 1)
echo "Migrations complete."
