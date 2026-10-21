#!/usr/bin/env bash
# One command to bring up the whole stack from a cold machine: Postgres and
# Redis (which need sudo to manage on this box, so they're not something
# dev-all.sh can start itself), then hands off to dev-all.sh for the
# backend, face service, and frontend. Asks for your sudo password once —
# the systemctl calls below reuse it.
#
# Our Postgres/Redis run on 5433/6380, not the defaults (5432/6379) — an
# unrelated Docker project on this machine ("algotest") squats the default
# ports with restart: unless-stopped, so ours moved off them. See .env and
# the comment above DB_PORT there for the full story.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/scripts/dev-env.sh"

echo "Starting Postgres…"
sudo systemctl start postgresql

echo "Starting Redis…"
# Idempotent: only touches a line that still says the default port, so
# re-running this after it's already been fixed is a harmless no-op.
sudo sed -i 's/^port 6379/port 6380/' /etc/redis/redis.conf
sudo systemctl start redis-server

exec "$REPO_ROOT/scripts/dev-all.sh"
