#!/usr/bin/env bash
# Boots a project-local .venv on first launch, then starts the MCP stdio server.
# Requires Python 3.11+ with the stdlib `venv` module — no global `uv` needed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENVDIR="$ROOT/.venv"
PY="$VENVDIR/bin/python"
PIP="$VENVDIR/bin/pip"

if [[ ! -x "$PY" ]]; then
  if ! command -v python3 >/dev/null 2>&1; then
    printf '%s\n' "memory MCP: python3 not found. Install Python 3.11+ (https://www.python.org/downloads/) and ensure python3 is on PATH." >&2
    exit 1
  fi
  if ! python3 -c 'import sys; sys.exit(0 if sys.version_info >= (3, 11) else 1)' 2>/dev/null; then
    printf '%s\n' "memory MCP: Python 3.11 or newer is required." >&2
    exit 1
  fi
  python3 -m venv "$VENVDIR"
  "$PIP" install -q --upgrade pip
  "$PIP" install -q -r "$ROOT/requirements.txt"
fi

exec "$PY" "$ROOT/server.py"
