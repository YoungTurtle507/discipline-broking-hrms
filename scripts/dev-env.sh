#!/usr/bin/env bash
# Sourced by the other dev scripts. Exports everything in .env so the Go
# backend, the face service, and migrate all read the same local config.
# Not meant to be run directly.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Go lives as a portable SDK in $HOME (not an apt install) and uv/migrate in
# ~/.local/bin and $GOPATH/bin. Ubuntu's ~/.bashrc returns early for
# non-interactive shells, so a script run outside a terminal won't have these
# on PATH — add them here rather than depending on the caller's environment.
for d in "$HOME/go-sdk/go/bin" "$HOME/go/bin" "$HOME/.local/bin"; do
  case ":$PATH:" in
    *":$d:"*) ;;
    *) [ -d "$d" ] && PATH="$d:$PATH" ;;
  esac
done
export PATH
export UV_SYSTEM_CERTS=1

if [ ! -f "$REPO_ROOT/.env" ]; then
  echo "No .env found at $REPO_ROOT/.env — copy .env.example to .env first." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source "$REPO_ROOT/.env"
set +a
