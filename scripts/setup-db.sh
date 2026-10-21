#!/usr/bin/env bash
# Creates the app's Postgres role and database, then verifies them.
#
# Usage:  sudo ./scripts/setup-db.sh
#
# Idempotent: safe to re-run. Reads credentials from .env so there is exactly
# one place defining them. Needs root only to reach the postgres superuser via
# local peer authentication.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

set -a
# shellcheck disable=SC1091
source "$REPO_ROOT/.env"
set +a

# Run psql as the postgres superuser, whichever way we have root.
if [ "$(id -u)" -eq 0 ]; then
  pg() { runuser -u postgres -- psql "$@"; }
else
  pg() { sudo -u postgres psql "$@"; }
fi

# cd away from the repo: the postgres user can't read it, and psql warns noisily.
cd /tmp

echo "Creating role ${POSTGRES_USER}..."
if [ "$(pg -tAc "SELECT 1 FROM pg_roles WHERE rolname='${POSTGRES_USER}'")" = "1" ]; then
  echo "  role already exists — updating its password to match .env"
  pg -v ON_ERROR_STOP=1 -c "ALTER ROLE ${POSTGRES_USER} LOGIN PASSWORD '${POSTGRES_PASSWORD}';"
else
  pg -v ON_ERROR_STOP=1 -c "CREATE ROLE ${POSTGRES_USER} LOGIN PASSWORD '${POSTGRES_PASSWORD}';"
  echo "  created"
fi

echo "Creating database ${POSTGRES_DB}..."
if [ "$(pg -tAc "SELECT 1 FROM pg_database WHERE datname='${POSTGRES_DB}'")" = "1" ]; then
  echo "  database already exists — leaving it alone"
else
  pg -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${POSTGRES_DB} OWNER ${POSTGRES_USER};"
  echo "  created"
fi

echo
echo "Verifying a real password login over TCP (what the Go backend does)..."
if PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "${DB_HOST}" -p "${DB_PORT}" \
     -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -tAc "SELECT 'connected as '||current_user||' to '||current_database();"
then
  echo
  echo "DB-SETUP-OK"
else
  echo
  echo "Role and database exist, but the TCP password login failed." >&2
  echo "Check /var/log/postgresql/postgresql-*-main.log for the reason." >&2
  exit 1
fi
