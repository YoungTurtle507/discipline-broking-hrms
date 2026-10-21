#!/usr/bin/env bash
# Usage: ./scripts/create-migration.sh <name>
# Creates backend/migrations/<seq>_<name>.up.sql and .down.sql
set -euo pipefail

if [ -z "${1:-}" ]; then
  echo "Usage: $0 <migration_name>"
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v migrate >/dev/null 2>&1; then
  echo "migrate not on PATH. Install it with:" >&2
  echo "  go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest" >&2
  echo "and make sure \$(go env GOPATH)/bin is on your PATH." >&2
  exit 1
fi

migrate create -ext sql -dir "$REPO_ROOT/backend/migrations" -seq "$1"
