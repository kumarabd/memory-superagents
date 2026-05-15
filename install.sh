#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BOLD='\033[1m'; NC='\033[0m'

info()  { echo -e "${GREEN}✓${NC} $*"; }
warn()  { echo -e "${YELLOW}!${NC} $*"; }
error() { echo -e "${RED}✗${NC} $*"; exit 1; }
step()  { echo -e "\n${BOLD}$*${NC}"; }

echo -e "${BOLD}Claude Memory — Installation${NC}"
echo "============================"; echo

# --- Prerequisites ---
step "Checking prerequisites..."
command -v docker  >/dev/null 2>&1 || error "docker is required. See https://docs.docker.com/get-docker/"
command -v python3 >/dev/null 2>&1 || error "Python 3.11+ is required: https://www.python.org/downloads/"
python3 -c 'import sys; sys.exit(0 if sys.version_info >= (3, 11) else 1)' 2>/dev/null || error "Python 3.11 or newer is required (found older python3)."
command -v go >/dev/null 2>&1 || error "go is required: https://go.dev/dl/"
if command -v claude >/dev/null 2>&1; then
  info "claude CLI on PATH — plugin will be registered with Claude Code."
else
  warn "claude CLI not on PATH — Postgres + CLI will install; add Claude Code and re-run this script (or run the plugin commands from the README) to install the plugin."
fi
info "All prerequisites found."

# --- Env vars ---
step "Configuring credentials..."
if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  read -rp "  Enter your OpenAI API key (sk-...): " OPENAI_API_KEY
  [[ -z "$OPENAI_API_KEY" ]] && error "OPENAI_API_KEY is required."
  export OPENAI_API_KEY
else
  info "OPENAI_API_KEY already set."
fi

DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/claude_memory}"
info "DATABASE_URL: $DATABASE_URL"

# --- Postgres ---
step "Starting PostgreSQL with pgvector..."
docker compose -f "$SCRIPT_DIR/docker-compose.yml" up -d
info "Container started."

step "Waiting for PostgreSQL to be ready..."
for i in $(seq 1 30); do
  if docker exec claude-memory-db pg_isready -U postgres -q 2>/dev/null; then
    info "PostgreSQL is ready."
    break
  fi
  [[ $i -eq 30 ]] && error "PostgreSQL did not become ready after 30 seconds."
  sleep 1
done

# --- CLI (build before local plugin install so ${CLAUDE_PLUGIN_ROOT}/cli/memory can exist) ---
step "Building memory CLI..."
command -v go >/dev/null 2>&1 || error "go is required: https://go.dev/dl/"
mkdir -p "$HOME/.local/bin"
(cd "$SCRIPT_DIR/cli" && go build -o "$SCRIPT_DIR/cli/memory" .)
cp -f "$SCRIPT_DIR/cli/memory" "$HOME/.local/bin/memory"
info "memory CLI installed to $SCRIPT_DIR/cli/memory (plugin hooks use this path) and ~/.local/bin/memory"

# --- Claude Code plugin (MCP + hooks via .mcp.json) ---
MEMORY_MARKETPLACE_URL="${MEMORY_MARKETPLACE_URL:-https://github.com/kumarabd/memory-superagents.git}"
MEMORY_PLUGIN_SELECTOR="${MEMORY_PLUGIN_SELECTOR:-claude-memory@claude-memory}"

step "Installing Claude Code plugin (claude-memory)..."
if [[ "${MEMORY_SKIP_CLAUDE_PLUGIN:-}" == "1" ]]; then
  warn "MEMORY_SKIP_CLAUDE_PLUGIN=1 — skipping claude plugin install."
elif ! command -v claude >/dev/null 2>&1; then
  warn "Skip plugin install (no claude on PATH). After installing Claude Code:"
  warn "  claude plugin marketplace add $MEMORY_MARKETPLACE_URL"
  warn "  claude plugin install $MEMORY_PLUGIN_SELECTOR --scope user"
else
  claude plugin marketplace add "$MEMORY_MARKETPLACE_URL" --scope user 2>/dev/null || true
  info "Marketplace source: $MEMORY_MARKETPLACE_URL (re-add is harmless if already registered)."
  if claude plugin install "$MEMORY_PLUGIN_SELECTOR" --scope user; then
    info "Plugin installed: $MEMORY_PLUGIN_SELECTOR (MCP + hooks from .mcp.json; no claude mcp add)."
  else
    warn "Marketplace install failed — installing from this directory instead..."
    if claude plugin install "$SCRIPT_DIR" --scope user; then
      info "Plugin installed from checkout: $SCRIPT_DIR"
    else
      warn "Plugin install failed. Install manually (README: Getting Started)."
    fi
  fi
fi

# --- Done ---
echo
echo -e "${BOLD}Installation complete!${NC}"
echo
echo "  Next steps:"
echo "    Restart Claude Code (or /reload-plugins) so the claude-memory plugin picks up MCP + hooks from .mcp.json."
echo "    memory doctor          # verify Postgres, schema, embeddings"
echo "    memory migrate         # apply SQL migrations (e.g. agentlab_notebook)"
echo "    memory status          # operational status"
echo
warn "Add these to your shell profile (~/.zshrc or ~/.bashrc) so the Claude app and hooks inherit them (GUI often does not read the terminal):"
echo "  export DATABASE_URL=\"$DATABASE_URL\""
echo "  export OPENAI_API_KEY=\"\$OPENAI_API_KEY\""
