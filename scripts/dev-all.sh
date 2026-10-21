#!/usr/bin/env bash
# Runs the backend, face service, and frontend together as one command.
# Each still logs to its own file under logs/ so you can tail one on its own;
# Ctrl-C here stops all three.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/scripts/dev-env.sh"

LOG_DIR="$REPO_ROOT/logs"
mkdir -p "$LOG_DIR"

BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="3100" # must match the -p flag in frontend/package.json's dev script
FACE_PORT="${FACE_SERVICE_PORT:-8001}"

pids=()

port_open() {
  (: >"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

cleaned_up=false
cleanup() {
  [ "$cleaned_up" = true ] && return
  cleaned_up=true
  echo
  echo "Stopping…"
  for pid in "${pids[@]:-}"; do
    [ -n "$pid" ] && kill "$pid" 2>/dev/null
  done
  wait 2>/dev/null
}
trap cleanup EXIT INT TERM

start() {
  local name="$1" port="$2" script="$3"
  if port_open "$port"; then
    echo "  $name: already running on port $port — leaving it alone"
    return
  fi
  "$script" >"$LOG_DIR/$name.log" 2>&1 &
  pids+=("$!")
  echo "  $name: starting (port $port, log: logs/$name.log)"
}

echo "Checking Postgres…"
if ! port_open "${DB_PORT:-5432}"; then
  echo "  Postgres isn't listening on ${DB_HOST:-localhost}:${DB_PORT:-5432} — start it first, e.g.:" >&2
  echo "    sudo systemctl start postgresql" >&2
  exit 1
fi
echo "  ok"

# Easy to miss otherwise: the backend starts and its own port opens fine
# even with no Redis at all (sessions/login just fail once someone actually
# tries to use it) — so without this check, everything below would report
# happily "up" while login is silently broken.
echo "Checking Redis…"
if ! port_open "${REDIS_PORT:-6379}"; then
  echo "  Redis isn't listening on ${REDIS_HOST:-localhost}:${REDIS_PORT:-6379} — start it first, e.g.:" >&2
  echo "    sudo systemctl start redis-server" >&2
  exit 1
fi
echo "  ok"

echo "Starting services…"
start backend "$BACKEND_PORT" "$REPO_ROOT/scripts/dev-backend.sh"
start face "$FACE_PORT" "$REPO_ROOT/scripts/dev-face.sh"
start frontend "$FRONTEND_PORT" "$REPO_ROOT/scripts/dev-frontend.sh"

echo
echo "Waiting for everything to come up (this can take a while the first time the face service loads its model)…"
all_up=true
for entry in "backend:$BACKEND_PORT" "face:$FACE_PORT" "frontend:$FRONTEND_PORT"; do
  name="${entry%%:*}"
  port="${entry##*:}"
  for _ in $(seq 1 90); do
    port_open "$port" && break
    sleep 1
  done
  if port_open "$port"; then
    echo "  $name: up"
  else
    echo "  $name: did NOT come up in time — check logs/$name.log" >&2
    all_up=false
  fi
done

cat <<EOF

HRMS:
  Frontend       https://localhost:$FRONTEND_PORT
  Backend API    http://localhost:$BACKEND_PORT
  Face service   http://localhost:$FACE_PORT

Logs: $LOG_DIR/{backend,face,frontend}.log
Press Ctrl+C to stop everything.
EOF

if [ "$all_up" = false ]; then
  echo "(one or more services failed to start — see the log noted above)" >&2
fi

if [ ${#pids[@]} -eq 0 ]; then
  # Everything was already running before this script started, so there's
  # nothing of ours to wait on — plain `wait` would return immediately and
  # fire the exit trap right away, printing "Stopping…" without having
  # stopped anything. Stay attached anyway so this window still does
  # something sensible when you Ctrl+C it.
  echo "(everything was already running — Ctrl+C here just closes this window)"
  while :; do sleep 3600; done
fi

wait
