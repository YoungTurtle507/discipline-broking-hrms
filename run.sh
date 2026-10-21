#!/usr/bin/env bash
# Starts the whole Discipline Broking HRMS: database, backend, face service,
# and frontend — one command, run from anywhere.
#
# Safe to run alongside the main HR SYSTEM at the same time — they share the
# same Postgres/Redis server on this machine, but use separate database
# names and a separate Redis index, and every port below (backend, face
# service, frontend) is different from the main HRMS's. They cannot collide.
#
# Will ask for your sudo password once (needed to start Postgres/Redis).
# Leave this window open while using the software; Ctrl+C here stops it.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Discipline Broking HRMS — isolated from the main HRMS (own database, own ports: 8180/8101/3100)"
echo

exec "$REPO_ROOT/scripts/dev-up.sh"
