#!/usr/bin/env bash
# Runs the Go API against the local Postgres/Redis. Ctrl-C to stop.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/scripts/dev-env.sh"

cd "$REPO_ROOT/backend"
exec go run ./cmd/server
