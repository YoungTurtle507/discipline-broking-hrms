#!/usr/bin/env bash
# Usage: ./scripts/db-migrate.sh [up|down 1|version|...]   (default: up)
# Applies backend/migrations to the local Postgres using the golang-migrate CLI.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/scripts/dev-env.sh"

if ! command -v migrate >/dev/null 2>&1; then
  echo "migrate not on PATH. Install it with:" >&2
  echo "  go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest" >&2
  echo "and make sure \$(go env GOPATH)/bin is on your PATH." >&2
  exit 1
fi

DB_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${DB_HOST}:${DB_PORT}/${POSTGRES_DB}?sslmode=disable"

exec migrate -path "$REPO_ROOT/backend/migrations" -database "$DB_URL" "${@:-up}"
