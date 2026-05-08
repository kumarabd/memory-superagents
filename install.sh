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
command -v go      >/dev/null 2>&1 || error "go is required: https://go.dev/dl/"
command -v claude  >/dev/null 2>&1 || error "claude CLI is required. Install Claude Code first."
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

# --- MCP server ---
step "Registering MCP server with Claude Code..."
claude mcp remove memory -s user 2>/dev/null || true
claude mcp add memory \
  -s user \
  -e DATABASE_URL="$DATABASE_URL" \
  -e OPENAI_API_KEY="$OPENAI_API_KEY" \
  -- /bin/bash \
     "$SCRIPT_DIR/mcp-server/run-mcp-server.sh"
info "MCP server registered."

# --- CLI ---
step "Building memory CLI..."
command -v go >/dev/null 2>&1 || error "go is required: https://go.dev/dl/"
(cd "$SCRIPT_DIR/cli" && go build -o "$HOME/.local/bin/memory" .)
info "memory CLI installed to ~/.local/bin/memory"

# --- Done ---
echo
echo -e "${BOLD}Installation complete!${NC}"
echo
echo "  Next steps:"
echo "    memory doctor        # verify everything is working"
echo "    memory status        # operational status"
echo
warn "Add these to your shell profile (~/.zshrc or ~/.bashrc) to persist across sessions:"
echo "  export DATABASE_URL=\"$DATABASE_URL\""
echo "  export OPENAI_API_KEY=\"\$OPENAI_API_KEY\""
