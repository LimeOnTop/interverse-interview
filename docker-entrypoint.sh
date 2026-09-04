#!/bin/sh
set -e

MIGRATE_PATH="${MIGRATE_PATH:-/migrations}"
MIGRATIONS_TABLE="${MIGRATIONS_TABLE:-schema_migrations}"

if [ -n "${DATABASE_URL:-}" ] && [ -d "$MIGRATE_PATH" ] && [ -n "$(ls -A "$MIGRATE_PATH" 2>/dev/null || true)" ]; then
  case "$DATABASE_URL" in
    *\?*) DB_URL="${DATABASE_URL}&x-migrations-table=${MIGRATIONS_TABLE}" ;;
    *)    DB_URL="${DATABASE_URL}?x-migrations-table=${MIGRATIONS_TABLE}" ;;
  esac

  echo "Running DB migrations (table=${MIGRATIONS_TABLE})..."
  migrate -path "$MIGRATE_PATH" -database "$DB_URL" up
  echo "Migrations complete."
else
  echo "Skipping migrations (DATABASE_URL or ${MIGRATE_PATH} missing)."
fi

exec "$@"
