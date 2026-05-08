# Plugin Restructuring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize the existing `memory-mcp-server` project into a Claude plugin layout installable with one command.

**Architecture:** Move all source files into a plugin-compliant directory structure, add a `.claude-plugin/` manifest, docker-compose for Postgres auto-migration, `install.sh` for guided setup, a SessionStart hook that activates memory loading automatically, and `.mcp.json` using `uv run` so no manual venv step is ever needed. The SKILL.md is bundled inside the plugin so it travels with the code.

**Tech Stack:** Python 3.11+, FastMCP, uv, Docker Compose, asyncpg, OpenAI embeddings.

---

### Task 1: Reorganize directory structure

**Files:**
- Move: `memory-mcp-server/` → `mcp-server/`
- Move: `schema.sql` → `migrations/001_initial.sql`
- Copy: `~/.claude/skills/memory-manager/SKILL.md` → `skills/memory-manager/SKILL.md`
- Create: `.claude-plugin/plugin.json`
- Create: `.claude-plugin/marketplace.json`

- [ ] **Step 1: Move mcp-server**

```bash
git mv memory-mcp-server mcp-server
```

- [ ] **Step 2: Create migrations directory, move schema**

```bash
mkdir migrations
git mv schema.sql migrations/001_initial.sql
```

- [ ] **Step 3: Bundle SKILL.md inside the plugin**

```bash
mkdir -p skills/memory-manager
cp ~/.claude/skills/memory-manager/SKILL.md skills/memory-manager/SKILL.md
git add skills/
```

- [ ] **Step 4: Create plugin manifest**

```bash
mkdir .claude-plugin
```

Create `.claude-plugin/plugin.json`:

```json
{
  "name": "claude-memory",
  "description": "Persistent memory layer for Claude Code — remembers preferences, decisions, and context across sessions using PostgreSQL + pgvector",
  "version": "0.1.0",
  "author": {
    "name": "Abishek Kumar",
    "email": "abishekkumar92@gmail.com"
  },
  "homepage": "https://github.com/your-org/claude-memory",
  "repository": "https://github.com/your-org/claude-memory",
  "license": "MIT",
  "keywords": ["memory", "persistence", "context", "agent", "pgvector"]
}
```

- [ ] **Step 5: Create marketplace.json**

Create `.claude-plugin/marketplace.json`:

```json
{
  "name": "claude-memory",
  "owner": {
    "name": "Abishek Kumar",
    "email": "abishekkumar92@gmail.com"
  },
  "plugins": [
    {
      "name": "claude-memory",
      "description": "Persistent memory layer for Claude Code",
      "version": "0.1.0",
      "source": ".",
      "author": {
        "name": "Abishek Kumar",
        "email": "abishekkumar92@gmail.com"
      }
    }
  ]
}
```

- [ ] **Step 6: Verify structure and commit**

```bash
ls -la
# Expected directories: .claude-plugin/  migrations/  mcp-server/  skills/
ls migrations/
# Expected: 001_initial.sql
ls skills/memory-manager/
# Expected: SKILL.md

git add .claude-plugin/
git commit -m "refactor: reorganize into plugin directory structure"
```

---

### Task 2: Create docker-compose.yml

**Files:**
- Create: `docker-compose.yml`

Placing migration SQL in `/docker-entrypoint-initdb.d` means it runs automatically on first `docker compose up` when the data volume is empty — no separate schema step on a fresh install.

- [ ] **Step 1: Create docker-compose.yml**

```yaml
version: "3.9"

services:
  postgres:
    image: ankane/pgvector
    container_name: claude-memory-db
    restart: unless-stopped
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: claude_memory
    ports:
      - "5432:5432"
    volumes:
      - claude_memory_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d

volumes:
  claude_memory_data:
```

- [ ] **Step 2: Verify compose file is valid**

```bash
docker compose config
# Expected: valid YAML output with no errors, no warnings
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "feat: add docker-compose for postgres+pgvector with auto-migration"
```

---

### Task 3: Create install.sh

**Files:**
- Create: `install.sh`

Does everything a user needs: checks prerequisites, starts Postgres, waits for readiness, registers the MCP server with Claude Code, installs the CLI, and tells the user what to add to their shell profile.

- [ ] **Step 1: Create install.sh**

```bash
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
command -v uv      >/dev/null 2>&1 || error "uv is required: curl -LsSf https://astral.sh/uv/install.sh | sh"
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
  -- uv \
     --directory "$SCRIPT_DIR/mcp-server" \
     run server.py
info "MCP server registered."

# --- CLI ---
step "Installing memory CLI..."
uv tool install --editable "$SCRIPT_DIR/cli"
info "memory CLI installed."

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
```

- [ ] **Step 2: Make executable and validate**

```bash
chmod +x install.sh
bash -n install.sh
# Expected: no output (no syntax errors)
```

- [ ] **Step 3: Commit**

```bash
git add install.sh
git commit -m "feat: add guided install.sh with prerequisite checks and MCP registration"
```

---

### Task 4: Create plugin hooks

**Files:**
- Create: `hooks/hooks.json`

The SessionStart hook fires when Claude Code starts. Its stdout is injected into Claude's context. This replaces the manual `~/.claude/CLAUDE.md` entry — installing the plugin is enough.

- [ ] **Step 1: Create hooks/hooks.json**

```bash
mkdir hooks
```

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'MEMORY SYSTEM ACTIVE. Invoke the claude-memory:memory-manager skill now. Load global preferences and workspace context before doing anything else.'",
            "async": false
          }
        ]
      }
    ]
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add hooks/
git commit -m "feat: add SessionStart hook — auto-activates memory-manager skill"
```

---

### Task 5: Create .mcp.json and update mcp-server for uv

**Files:**
- Create: `.mcp.json`
- Modify: `mcp-server/pyproject.toml`

`.mcp.json` is read by Claude Code when the plugin is installed. Uses `uv run` which auto-creates a venv and installs deps from `pyproject.toml` on first run — no manual pip step needed.

- [ ] **Step 1: Create .mcp.json**

```json
{
  "memory": {
    "command": "uv",
    "args": [
      "--directory",
      "${CLAUDE_PLUGIN_ROOT}/mcp-server",
      "run",
      "server.py"
    ],
    "env": {
      "DATABASE_URL": "${DATABASE_URL}",
      "OPENAI_API_KEY": "${OPENAI_API_KEY}"
    }
  }
}
```

- [ ] **Step 2: Validate JSON**

```bash
python3 -c "import json; json.load(open('.mcp.json')); print('valid')"
# Expected: valid
```

- [ ] **Step 3: Add [tool.uv] section to mcp-server/pyproject.toml**

Full content of `mcp-server/pyproject.toml`:

```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "memory-mcp-server"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = [
  "fastmcp>=2.0",
  "asyncpg>=0.29",
  "openai>=1.0",
  "pydantic>=2.0",
]

[tool.uv]
dev-dependencies = []
```

- [ ] **Step 4: Test uv can resolve and run the server**

```bash
DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
OPENAI_API_KEY="placeholder" \
uv --directory mcp-server run python -c "import server; print('imports OK')"
# Expected: imports OK
```

- [ ] **Step 5: Commit**

```bash
git add .mcp.json mcp-server/pyproject.toml
git commit -m "feat: add .mcp.json for plugin MCP registration via uv run"
```

---

### Task 6: Update README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Replace Getting Started section**

The new Getting Started section content:

```markdown
## Getting Started

### Prerequisites

- Docker + Docker Compose
- [uv](https://docs.astral.sh/uv/install/) — `curl -LsSf https://astral.sh/uv/install.sh | sh`
- Claude Code CLI
- OpenAI API key

### Option A — Plugin install (recommended)

```bash
# Add this repo as a local marketplace
claude plugin marketplace add /path/to/claude-memory

# Install the plugin
claude plugin install claude-memory@claude-memory
```

Then run the guided setup:

```bash
OPENAI_API_KEY=sk-... ./install.sh
```

### Option B — Manual install

```bash
git clone https://github.com/your-org/claude-memory
cd claude-memory
OPENAI_API_KEY=sk-... ./install.sh
```

### Verify

```bash
memory doctor
```

### Daily use

Memory loads automatically at every Claude Code session start. You can also inspect and manage it:

```bash
memory status
memory search "kubernetes preferences"
memory stats
memory export --format timeline
```
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README for plugin install and install.sh workflow"
```

---

### Final structure after this plan

```
claude-memory/               (was: memory-superagents/)
├── .claude-plugin/
│   ├── plugin.json
│   └── marketplace.json
├── .mcp.json
├── docker-compose.yml
├── install.sh
├── hooks/
│   └── hooks.json
├── migrations/
│   └── 001_initial.sql      (was: schema.sql)
├── mcp-server/              (was: memory-mcp-server/)
│   ├── db.py
│   ├── embeddings.py
│   ├── models.py
│   ├── server.py
│   └── pyproject.toml
├── skills/
│   └── memory-manager/
│       └── SKILL.md
└── README.md
```

The `cli/` directory is built in Plan 2 (memory-cli).
