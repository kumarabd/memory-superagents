#!/usr/bin/env bash
# Resolve the memory CLI for Claude Code plugin hooks: prefer plugin-local
# binary (${CLAUDE_PLUGIN_ROOT}/cli/memory), then PATH. Fail-open so a missing
# build never blocks SessionStart / SessionEnd.
set -euo pipefail

subcmd="${1:-session-start}"
root="${CLAUDE_PLUGIN_ROOT:-}"
if [[ -z "$root" ]]; then
  root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fi

local_bin="${root}/cli/memory"
if [[ -x "$local_bin" ]]; then
  exec "$local_bin" hook "$subcmd"
fi

if command -v memory >/dev/null 2>&1; then
  exec memory hook "$subcmd"
fi

msg="Claude Memory: memory CLI not found. From the plugin directory run ./install.sh (builds cli/memory) or: (cd cli && go build -o memory .)"
if [[ "$subcmd" == "session-start" ]]; then
  printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"Claude Memory: memory CLI not found. From the plugin directory run ./install.sh (builds cli/memory) or: cd cli && go build -o memory ."}}'
else
  echo "$msg" >&2
fi
exit 0
