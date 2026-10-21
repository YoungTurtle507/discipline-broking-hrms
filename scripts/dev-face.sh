#!/usr/bin/env bash
# Runs the Python face service. Expects face-service/venv to exist — see
# scripts/setup-face-service.sh if it doesn't.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/scripts/dev-env.sh"

cd "$REPO_ROOT/face-service"
if [ ! -x venv/bin/uvicorn ]; then
  echo "face-service/venv is missing or incomplete — run scripts/setup-face-service.sh" >&2
  exit 1
fi

# Port must match FACE_SERVICE_URL, which is what the Go backend calls.
exec venv/bin/uvicorn api.main:app --host 0.0.0.0 --port "${FACE_SERVICE_PORT:-8001}" --reload
