#!/usr/bin/env bash
# Runs the Next.js dev server over HTTPS (required for camera access from a LAN
# IP — see next.config.ts).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$REPO_ROOT/frontend"
if [ ! -d node_modules ]; then
  echo "frontend/node_modules is missing — run npm install first." >&2
  exit 1
fi

exec npm run dev
