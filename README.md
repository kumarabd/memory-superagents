# claude-memory

**Repository:** [https://github.com/kumarabd/memory-superagents](https://github.com/kumarabd/memory-superagents)

A persistent memory layer for Claude Code. Gives Claude the ability to remember preferences, decisions, problems, solutions, and context across sessions and workspaces — using PostgreSQL with pgvector for semantic search.

## How it works

The MCP server exposes **core** tools (`memory.*`) for capture and retrieval plus **insights** tools (`insights.*`) for aggregates and derived rows stored with lineage. Claude writes and retrieves typed memory events primarily via core tools. Every memory has a type, scope, importance, confidence, and optional project tag. Semantic search is powered by OpenAI embeddings (`text-embedding-ada-002`) stored in a `vector(1536)` column.

**Recommended setup:** clone [memory-superagents](https://github.com/kumarabd/memory-superagents) and run **`./install.sh`**: it starts Postgres, builds **`cli/memory`**, and (when **`claude`** is on **`PATH`**) registers the GitHub marketplace and runs **`claude plugin install claude-memory@claude-memory --scope user`** so MCP + hooks load from **`.mcp.json`** (no **`claude mcp add`**). Set **`MEMORY_SKIP_CLAUDE_PLUGIN=1`** to skip plugin commands (e.g. CI). Override defaults with **`MEMORY_MARKETPLACE_URL`** and **`MEMORY_PLUGIN_SELECTOR`**. Persist **`DATABASE_URL`** in your shell so SessionStart hooks and the MCP process match.

At the start of each Claude Code session the **SessionStart** hook runs **`memory hook session-start`** (via the wrapper above), which (1) exports session metadata into `CLAUDE_ENV_FILE` when present, and (2) when **`DATABASE_URL`** is set, performs the same **first-touch materialization** as MCP **`notebook.load`** for the session workspace path (`cwd` from the hook payload, or **`CLAUDE_PROJECT_DIR`** if `cwd` is empty): ensures a row exists in **`agentlab_notebook`** so `SELECT * FROM agentlab_notebook` and subsequent **`notebook.load`** calls see that workspace without waiting for an agent tool call.

The memory-manager skill still applies on top of that hook-driven setup:

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
│   ├── hooks.json           # SessionStart / SessionEnd → memory-hook.sh
│   └── memory-hook.sh       # Prefers ${CLAUDE_PLUGIN_ROOT}/cli/memory, then PATH
├── migrations/
│   ├── 001_initial.sql
│   └── 002_agentlab_notebook.sql   # AgentLab notebook table
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

### AgentLab notebook (`notebook.*`)

Structured workspace memory for **AgentLab** (`superagents`): one JSON payload per workspace (absolute path as `project`). Backed by table **`agentlab_notebook`**. Apply migrations with **`memory migrate`** after pulling (adds `002_agentlab_notebook.sql`).

| Tool | Purpose |
|------|---------|
| `notebook.load` | Return `{ workspace_key, version, updated_at, notebook }` for the workspace. **First call** inserts a default row (so `SELECT * FROM agentlab_notebook` shows your workspace after one load). |
| `notebook.patch` | Replace any provided top-level notebook keys (`term_cache`, `findings`, `preferences`, …); optional `expected_version` for optimistic locking |

**`agentlab_notebook` lives only in this DB** (the one in `DATABASE_URL` / docker-compose `claude_memory`, typically port **5432**). It is **not** on AgentLab’s hydrate DB (**5433** / `agentlab_environment`). If you `psql` the wrong server, the table will be missing or always empty.

**No rows after setup?**

1. Run **`memory migrate`** so `002_agentlab_notebook.sql` is applied (`\dt agentlab_notebook` in `claude_memory`).
2. Confirm the memory MCP **`DATABASE_URL`** matches the DB you are inspecting.
3. In Claude, call **`notebook.load`** once with **`project`** = the **exact absolute path** of the workspace (same string you will use in `notebook.patch`). Then re-query:

   ```bash
   psql "$DATABASE_URL" -c "SELECT workspace_key, version FROM agentlab_notebook;"
   ```

4. If `notebook.load` still returns `version: 0` and no `_proxy` error, the MCP process may not have `DATABASE_URL` set (confirm env in Claude’s **MCP / plugin** settings and in your shell profile).

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
- Claude Code (the **claude-memory** plugin supplies MCP via **`.mcp.json`** — do not run `claude mcp add` for memory unless you intentionally want a duplicate user-scope entry under `~/.claude`)
- Go (for building the optional `memory` CLI via `install.sh`)
- OpenAI API key

The MCP server installs its own dependencies into `mcp-server/.venv/` the first time it starts (`run-mcp-server.sh`). Developers may still use [uv](https://docs.astral.sh/uv/) with `uv run server.py` if they prefer.

### 1 — Install from GitHub (script or manual)

**Single command (recommended):** clone the repo and run **`./install.sh`**. With **`claude`** on your **`PATH`**, the script registers **`https://github.com/kumarabd/memory-superagents.git`** as a marketplace and installs **`claude-memory@claude-memory`** (user scope). It also starts Postgres and builds the **`memory`** CLI. Skip plugin steps with **`MEMORY_SKIP_CLAUDE_PLUGIN=1`**.

```bash
git clone https://github.com/kumarabd/memory-superagents.git
cd memory-superagents
OPENAI_API_KEY=sk-... ./install.sh
```

**Manual (same as the script):** in Claude Code or the terminal:

```text
/plugin marketplace add https://github.com/kumarabd/memory-superagents.git
/plugin install claude-memory@claude-memory
/reload-plugins
```

```bash
claude plugin marketplace add https://github.com/kumarabd/memory-superagents.git
claude plugin install claude-memory@claude-memory --scope user
```

The marketplace **`name`** in [`.claude-plugin/marketplace.json`](.claude-plugin/marketplace.json) is **`claude-memory`**; the plugin id is **`claude-memory`**, so the install selector is **`claude-memory@claude-memory`**.

### 2 — If you skipped the script’s plugin step

If **`claude`** was missing during **`./install.sh`**, run the marketplace + install commands above after installing Claude Code, **or** re-run **`./install.sh`** from the clone.

### 3 — Data plane only (already covered by `./install.sh`)

The script runs **`docker compose`**, builds **`cli/memory`** under the clone, and copies it to **`~/.local/bin/memory`**. When the plugin is loaded from Claude’s cache, **`hooks/memory-hook.sh`** falls back to **`PATH`** if the cache tree has no **`cli/memory`** yet — keep **`~/.local/bin`** on **`PATH`**.

### Alternative — Local scope or forks

**`./install.sh`** falls back to **`claude plugin install "$SCRIPT_DIR" --scope user`** if **`claude-memory@claude-memory`** fails (e.g. offline or fork). For **project**/**`local`** scope only, run **`claude plugin install "$(pwd)" --scope local`** yourself after **`./install.sh`**.

If you still have a **duplicate** `memory` entry from an older **`install.sh`** (`claude mcp add -s user`), remove it once: **`claude mcp remove memory -s user`**, then rely on the plugin only.

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
| `MEMORY_SKIP_CLAUDE_PLUGIN` | Set to `1` to skip `claude plugin marketplace add` / `install` inside `./install.sh` |
| `MEMORY_MARKETPLACE_URL` | Override default `https://github.com/kumarabd/memory-superagents.git` |
| `MEMORY_PLUGIN_SELECTOR` | Override default `claude-memory@claude-memory` |

---

## Keeping PostgreSQL running across reboots

The docker-compose.yml sets `restart: unless-stopped`, so the container restarts automatically after a reboot as long as Docker Desktop is set to start on login.
