#!/usr/bin/env bash
# Creates face-service/venv on Python 3.12 and installs requirements.txt.
#
# Why 3.12 and not the system Python: Ubuntu 26.04 ships only Python 3.14, and
#   (a) onnxruntime 1.20.1 publishes no 3.14 wheels, and
#   (b) Python 3.13+ enables ssl.VERIFY_X509_STRICT by default, which rejects
#       this network's SonicWALL DPI-SSL certificate (its basicConstraints
#       extension isn't marked critical), so pip can't reach PyPI at all.
# uv fetches a standalone 3.12 build without touching the system Python.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT/face-service"

if ! command -v uv >/dev/null 2>&1; then
  cat >&2 <<'MSG'
uv is not on PATH. Install it with:

  curl -LsSf https://astral.sh/uv/install.sh | sh
  export PATH="$HOME/.local/bin:$PATH"

then re-run this script.
MSG
  exit 1
fi

# Use the system trust store (which includes the local TLS-inspection CA)
# instead of uv's bundled webpki roots.
export UV_NATIVE_TLS=1

uv python install 3.12

rm -rf venv
uv venv --python 3.12 venv

venv_reqs="$(mktemp)"
trap 'rm -f "$venv_reqs"' EXIT

if ! uv pip install --python venv/bin/python -r requirements.txt; then
  echo
  echo "Full install failed — retrying with the locally patched insightface in vendor/." >&2
  echo "(That copy drops the face3d mesh_core_cython C++ extension, which this" >&2
  echo " project never uses; only 2D detection + ArcFace ONNX embeddings, and" >&2
  echo " building it needs a C++ compiler.)" >&2
  echo
  # Everything except insightface comes from PyPI as normal; insightface comes
  # from the patched local copy. The filtered list goes through a file so each
  # pin stays a separate argument.
  grep -v '^insightface' requirements.txt > "$venv_reqs"
  uv pip install --python venv/bin/python -r "$venv_reqs"
  uv pip install --python venv/bin/python ./vendor/insightface-0.7.3
fi

echo
echo "Done. venv/bin/python is $(venv/bin/python -V)"
echo "Note: the first face-service start downloads the buffalo_l model pack"
echo "(~300MB) into ~/.insightface — expect a slow first boot."
