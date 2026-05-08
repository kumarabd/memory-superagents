# Memory MCP Server — Design Spec

**Date:** 2026-05-07  
**Status:** Approved

---

## Overview

A Python MCP server that exposes a persistent memory layer backed by PostgreSQL (with pgvector) to Claude Code. Claude can store and retrieve typed memory events across sessions, projects, and topics using semantic vector search and structured SQL queries.

Registration:
```bash
claude mcp add memory -e DATABASE_URL="postgres://..." -e OPENAI_API_KEY="sk-..." -- python /path/to/memory-mcp-server/server.py
```

---

## File Structure

```
memory-mcp-server/
├── server.py        # FastMCP app + all @mcp.tool() definitions
├── db.py            # asyncpg pool, all SQL query functions
├── embeddings.py    # OpenAI client, embed() helper
├── models.py        # MemoryType enum, SearchFilters Pydantic model
└── pyproject.toml   # fastmcp, asyncpg, openai, pydantic
```

### Module responsibilities

| Module | Responsibility | Imports |
|---|---|---|
| `models.py` | 25-type enum, input/output Pydantic models | none (stdlib + pydantic only) |
| `embeddings.py` | `embed(text) -> list[float]` via OpenAI | `openai`, `models` |
| `db.py` | asyncpg pool, all SQL query functions | `asyncpg`, `models` |
| `server.py` | FastMCP app, tool wiring | `fastmcp`, `db`, `embeddings`, `models` |

---

## Configuration

| Env var | Required | Description |
|---|---|---|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `OPENAI_API_KEY` | Yes | For `text-embedding-ada-002` embedding generation |

Both are validated at startup. The server exits with a clear error message if either is missing — no runtime failures.

---

## Database Schema

Three tables (defined in `schema.sql`):

- **`conversation_sessions`** — one row per Claude Code session; carries `workspace_path` used as the project identifier
- **`messages`** — per-message log (not used by any tool in this build)
- **`memory_events`** — the core table; every memory write lands here

`memory_events` key columns:
- `memory_type` — constrained to the 25-type taxonomy (see below)
- `embedding vector(1536)` — generated via `text-embedding-ada-002`
- `session_id` — foreign key to `conversation_sessions`; project-scoped queries JOIN through this

**Schema decision:** `workspace_path` is NOT duplicated onto `memory_events`. Project-scoped queries require a `session_id` link. Orphan writes (no session) are not project-queryable — acceptable for this build.

---

## Memory Type Taxonomy

25 types enforced via a CHECK constraint on `memory_events.memory_type`:

| Type | Purpose |
|---|---|
| `preference` | Likes/dislikes, defaults, style choices |
| `profile_fact` | Stable facts about the user, environment, tools, constraints |
| `project_context` | Goals, scope, architecture, current status of a project |
| `decision` | Chosen direction + rationale + alternatives rejected |
| `task` | Pending work, follow-ups |
| `event` | Something that happened: deployment, outage, purchase, travel |
| `problem` | Bug, error, blocker, recurring pain point |
| `solution` | Fixes, commands, runbooks, procedures that worked |
| `learning` | Concepts explained, mental models built |
| `question` | Open questions or unresolved research threads |
| `plan` | Step-by-step strategy, roadmap, implementation plan |
| `constraint` | Hard limits: budget, time, infra, legal, hardware, policy |
| `credential_reference` | References to secrets (not the secrets themselves) |
| `relationship` | People, orgs, contacts, roles, communication preferences |
| `routine` | Repeated workflows, habits, schedules, recurring checklists |
| `artifact` | Files, repos, docs, diagrams, scripts, links, outputs |
| `conversation_summary` | Compressed summary of a session or topic thread |
| `correction` | "Actually that was wrong" or "use X instead of Y" |
| `feedback` | Rating of an answer, tool, plan, product, or decision |
| `observation` | Raw noteworthy evidence: logs, measurements, benchmarks |
| `hypothesis` | Possible explanation or idea not yet validated |
| `experiment` | Test setup, result, conclusion |
| `capability` | What an agent/tool/system can or cannot do |
| `policy` | Rules for behavior, safety, approval, execution, spending |
| `identity` | How an assistant should behave: tone, role, boundaries |

---

## Tools

### `memory.search(query, filters)`

Semantic search across all memory events.

**Inputs:**
- `query: str` — natural language search query
- `filters: SearchFilters` — optional structured filters:
  ```python
  class SearchFilters(BaseModel):
      memory_type: MemoryType | None = None
      scope: str | None = None          # defaults to 'personal'
      project: str | None = None        # workspace_path
  ```

**Behavior:**
1. Embed `query` via `embed()`
2. Run pgvector cosine similarity search with optional WHERE clauses
3. JOIN `conversation_sessions` when `project` filter is set
4. Return top 10 results ordered by similarity, including `similarity` score

**SQL pattern:**
```sql
SELECT m.*, 1 - (m.embedding <=> $1::vector) AS similarity
FROM memory_events m
LEFT JOIN conversation_sessions s ON m.session_id = s.id
WHERE ($2::text IS NULL OR m.memory_type = $2)
  AND ($3::text IS NULL OR m.scope = $3)
  AND ($4::text IS NULL OR s.workspace_path = $4)
ORDER BY m.embedding <=> $1::vector
LIMIT 10
```

---

### `memory.write(type, content, metadata)`

Write a new memory event.

**Inputs:**
- `type: MemoryType` — one of the 25 types; validated by Pydantic before hitting the DB
- `content: str` — the memory content
- `metadata: dict` — arbitrary JSON; can include `importance`, `confidence`, `scope`, `subject` overrides

**Behavior:**
1. Validate `type` against `MemoryType` enum (Pydantic rejects invalid values before DB call)
2. Embed `content` via `embed()`
3. Insert into `memory_events` with defaults: `importance=0.5`, `confidence=0.7`, `scope='personal'`
4. Overrides from `metadata` (e.g. `{"importance": 0.9, "subject": "prefers dark mode"}`) are applied before insert
5. Returns the inserted row's `id`

---

### `memory.recent(project, limit)`

Fetch the most recently written memory events for a project.

**Inputs:**
- `project: str` — `workspace_path` of the Claude Code session
- `limit: int` — defaults to 20

**Behavior:**
- JOIN `conversation_sessions` on `workspace_path = project`
- Order by `memory_events.created_at DESC`
- Return `id`, `memory_type`, `subject`, `content`, `importance`, `created_at`
- No embedding needed — pure SQL

---

### `memory.get_decisions(project)`

Fetch all decisions recorded for a project.

**Inputs:**
- `project: str` — `workspace_path`

**Behavior:**
- Equivalent to `memory.search` with `filters.memory_type = 'decision'` and `filters.project = project` but without vector ranking — returns all decisions ordered newest-first
- Returns `id`, `subject`, `content`, `metadata`, `created_at`

---

### `memory.get_preferences(topic)`

Semantic search scoped to preferences.

**Inputs:**
- `topic: str` — natural language description of the preference topic

**Behavior:**
- Equivalent to `memory.search` with `filters.memory_type = 'preference'` hardcoded
- Embeds `topic`, runs vector search within the preference subset
- Returns top 10 by similarity

---

## Embedding Strategy

- **Model:** `text-embedding-ada-002` (1536 dimensions, matches schema)
- **Function:** `embed(text: str) -> list[float]` in `embeddings.py`
- **Called by:** `memory.search`, `memory.write`, `memory.get_preferences`
- **On failure:** raises an MCP error with the OpenAI error message; no silent fallback

---

## Error Handling

| Scenario | Behavior |
|---|---|
| Missing `DATABASE_URL` or `OPENAI_API_KEY` | Startup failure with clear message |
| Invalid `memory_type` | Pydantic validation error before DB call |
| OpenAI API failure | MCP tool error with OpenAI message |
| DB query failure | MCP tool error; asyncpg pool handles reconnection |
| No results found | Returns empty list (not an error) |

---

## Dependencies

```toml
[project]
dependencies = [
  "fastmcp>=2.0",
  "asyncpg>=0.29",
  "openai>=1.0",
  "pydantic>=2.0",
]
```

---

## Out of Scope

- `messages` table — not used by any tool in this build
- Session management — `memory.write` creates orphan events; session tracking is a future concern
- Authentication / multi-user isolation
- Test suite — verification via Claude Code after registration
