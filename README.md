# claude-memory

A persistent memory layer for Claude Code. Gives Claude the ability to remember preferences, decisions, problems, solutions, and context across sessions and workspaces — using PostgreSQL with pgvector for semantic search.

## How it works

Claude writes and retrieves typed memory events via six MCP tools. Every memory has a type, scope, importance, confidence, and optional project tag. Semantic search is powered by OpenAI embeddings (`text-embedding-ada-002`) stored in a `vector(1536)` column.

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
├── .mcp.json                # MCP server registration (uv run)
├── docker-compose.yml       # PostgreSQL + pgvector
├── install.sh               # Guided setup script
├── hooks/
│   └── hooks.json           # SessionStart hook
├── migrations/
│   └── 001_initial.sql      # Database schema
├── mcp-server/              # FastMCP Python server
│   ├── db.py
│   ├── embeddings.py
│   ├── models.py
│   ├── server.py
│   └── pyproject.toml
├── skills/
│   └── memory-manager/
│       └── SKILL.md         # Bundled skill
└── cli/                     # memory CLI (operational control plane)
    └── memory_cli/
```

## MCP Tools

| Tool | Purpose |
|---|---|
| `memory.search` | Semantic search with optional type/scope/project filters |
| `memory.write` | Write any memory event with full metadata |
| `memory.recent` | Most recent events for a project, newest first |
| `memory.get_decisions` | All active decisions for a workspace |
| `memory.get_project_context` | Project context memories for a workspace |
| `memory.get_preferences` | Semantic search over personal/global preferences |

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

---

## Environment Variables

| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string, e.g. `postgres://user:pass@host:5432/dbname` |
| `OPENAI_API_KEY` | OpenAI API key — used for generating embeddings via `text-embedding-ada-002` |

---

## Keeping PostgreSQL running across reboots

The docker-compose.yml sets `restart: unless-stopped`, so the container restarts automatically after a reboot as long as Docker Desktop is set to start on login.
