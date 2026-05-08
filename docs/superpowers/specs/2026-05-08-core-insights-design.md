# Core vs Insights — Design Spec

**Date:** 2026-05-08  
**Status:** Approved (persistence model **A**)  

---

## Goals

Split the Claude memory platform into:

| Layer | Responsibility |
|---|---|
| **Core** | Capture, persist, retrieve individual memory events (`memory.*` MCP tools). |
| **Insights** | Analyze aggregates, derive structured views, expose data suitable for visualization, and persist **synthesized** knowledge back into the same `memory_events` table with explicit **lineage**. |

Single MCP server process (`server.py`): shared DB pool and embeddings client; tooling registered from `core` and `insights` modules.

---

## Persistence model (choice **A**)

Derived insight rows are ordinary **`memory_events`** rows:

- Prefer `memory_type` **`learning`** for cross-session pattern summaries; other types (e.g. `conversation_summary`, `observation`) remain available via `memory.write` when appropriate.
- **Lineage** lives in **`metadata`** (JSONB), not new tables.
- Required lineage shape for tool-written syntheses:

```json
{
  "lineage": {
    "tool": "<insights tool name, e.g. insights.persist_synthesis>",
    "source_memory_ids": ["<uuid>", "..."]
  },
  "created_from": "derived",
  "project": "<workspace_path>",
  "status": "active"
}
```

Callers may add `topic`, `tags`, etc. Queries that need "only raw captures" can filter with `metadata->>'created_from' != 'derived'` later if desired (not enforced in v1).

---

## Repository layout (`mcp-server/`)

```
mcp-server/
├── server.py           # FastMCP app, lifespan, registers core + insights
├── common/
│   ├── db.py           # asyncpg pool, SQL (core reads/writes + insight aggregates)
│   ├── embeddings.py
│   └── models.py       # MemoryType, SearchFilters
├── core/
│   └── tools.py       # memory.search | write | recent | get_* 
└── insights/
    └── tools.py       # insights.* analyze / persist synthesized rows
```

---

## MCP tools

### Core (unchanged names)

Existing `memory.*` tools behave as today; implementation lives under `core/tools.py`.

### Insights (initial surface)

| Tool | Purpose |
|---|---|
| `insights.project_distribution` | Read-only counts by `memory_type` for a workspace, plus total and first/last `created_at`. Chart-friendly JSON. |
| `insights.persist_synthesis` | Embeds `content`, inserts `learning` row with scope `project`, metadata including `lineage.source_memory_ids` and `created_from: derived`. |

Future tools (explain/narrative, richer viz payloads, global aggregates) extend `insights/tools.py` and `common/db.py` without renaming `memory.*`.

---

## CLI / hooks / skills

- **CLI:** May later mirror groups (`memory insights …`); not required for this structural change.
- **Hooks:** unchanged.
- **Skill:** Agents should prefer `insights.project_distribution` before manual counting; use `insights.persist_synthesis` when consolidating patterns with traceable sources.

---

## Testing

Smoke: `uv run python -c "import server"` from `mcp-server/` with env vars unset may fail at run() only; import graph must load without DB connection.
