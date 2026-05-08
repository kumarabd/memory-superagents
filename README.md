# claude-memory

A persistent memory layer for Claude Code. Gives Claude the ability to remember preferences, decisions, problems, solutions, and context across sessions and workspaces — using PostgreSQL with pgvector for semantic search.

## How it works

The MCP server exposes **core** tools (`memory.*`) for capture and retrieval plus **insights** tools (`insights.*`) for aggregates and derived rows stored with lineage. Claude writes and retrieves typed memory events primarily via core tools. Every memory has a type, scope, importance, confidence, and optional project tag. Semantic search is powered by OpenAI embeddings (`text-embedding-ada-002`) stored in a `vector(1536)` column.

At the start of each Claude Code session the memory-manager skill fires automatically via a `SessionStart` hook, loading two tracks of context:

- **Track A — Global:** preferences and personal facts that apply across all workspaces
- **Track B — Project:** decisions, context, recent events for the current workspace

Dead memories (status `superseded` or `stale`) are excluded from all queries by default.

## Architecture

```
claude-memory/
├── .claude-plugin/
│   ├── plugin.json          # Claude plugin manifest
│   └── marketplace.json     # Marketplace registration
├── .mcp.json                # MCP registration (bootstrap venv via run-mcp-server.sh)
├── docker-compose.yml       # PostgreSQL + pgvector
├── install.sh               # Guided setup script
├── hooks/
│   └── hooks.json           # SessionStart hook
├── migrations/
│   └── 001_initial.sql      # Database schema
├── mcp-server/              # FastMCP Python server
│   ├── common/              # shared DB pool, embeddings, models
│   ├── core/                # memory.* (capture / store / retrieve)
│   ├── insights/            # insights.* (analyze / derive / viz-friendly stats)
│   ├── server.py
│   └── pyproject.toml
├── skills/
│   └── memory-manager/
│       └── SKILL.md         # Bundled skill
└── cli/                     # memory CLI (operational control plane)
    └── memory_cli/
```

## MCP Tools

### Core (`memory.*`)

| Tool | Purpose |
|---|---|
| `memory.search` | Semantic search with optional type/scope/project filters |
| `memory.write` | Write any memory event with full metadata |
| `memory.recent` | Most recent events for a project, newest first |
| `memory.get_decisions` | All active decisions for a workspace |
| `memory.get_project_context` | Project context memories for a workspace |
| `memory.get_preferences` | Semantic search over personal/global preferences |

### Insights (`insights.*`)

| Tool | Purpose |
|---|---|
| `insights.project_distribution` | Counts by memory type per workspace plus date span (for charts/tables) |
| `insights.persist_synthesis` | Save a derived pattern as `learning` with `metadata.lineage.source_memory_ids` |

## Memory CLI

The `memory` command is the operational control plane:

```bash
memory doctor       # full system health check
memory status       # DB size, counts, last write
memory stats        # analytics dashboard by type/topic/project
memory search "query"  # semantic search
memory export --format timeline  # month-by-month cognitive timeline
memory backup       # pg_dump to file
memory compact      # archive stale memories
memory reindex      # re-embed after model change
```

## Memory Types

25 typed categories: `preference`, `profile_fact`, `project_context`, `decision`, `task`, `event`, `problem`, `solution`, `learning`, `question`, `plan`, `constraint`, `credential_reference`, `relationship`, `routine`, `artifact`, `conversation_summary`, `correction`, `feedback`, `observation`, `hypothesis`, `experiment`, `capability`, `policy`, `identity`.

---

## Getting Started

### Prerequisites

- Docker + Docker Compose
- **Python 3.11+** on `PATH` as `python3` (stdlib `venv` is enough — **no global `uv` required**)
- Claude Code CLI
- Go (for building the optional `memory` CLI via `install.sh`)
- OpenAI API key

The MCP server installs its own dependencies into `mcp-server/.venv/` the first time it starts (`run-mcp-server.sh`). Developers may still use [uv](https://docs.astral.sh/uv/) with `uv run server.py` if they prefer.

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

To register the MCP server by hand (same as `install.sh` does), use the bootstrap script so the repo does not depend on global `uv`:

```bash
claude mcp add memory -s user \
  -e DATABASE_URL="postgres://postgres:postgres@localhost:5432/claude_memory" \
  -e OPENAI_API_KEY="sk-..." \
  -- /bin/bash /path/to/claude-memory/mcp-server/run-mcp-server.sh
```

The first successful start may take a short while while `pip` fills `mcp-server/.venv/`.

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

---

## Environment Variables

| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string, e.g. `postgres://user:pass@host:5432/dbname` |
| `OPENAI_API_KEY` | OpenAI API key — used for generating embeddings via `text-embedding-ada-002` |

---

## Keeping PostgreSQL running across reboots

The docker-compose.yml sets `restart: unless-stopped`, so the container restarts automatically after a reboot as long as Docker Desktop is set to start on login.
