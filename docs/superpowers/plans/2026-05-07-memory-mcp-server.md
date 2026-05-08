# Memory MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a FastMCP Python server exposing 5 memory tools (`memory.search`, `memory.write`, `memory.recent`, `memory.get_decisions`, `memory.get_preferences`) backed by PostgreSQL with pgvector.

**Architecture:** Four focused modules (`models`, `embeddings`, `db`, `server`) wired by a FastMCP app. `DATABASE_URL` and `OPENAI_API_KEY` are validated at startup. All SQL lives in `db.py`; all OpenAI calls in `embeddings.py`. Server runs in stdio mode for Claude Code MCP registration.

**Tech Stack:** Python 3.11+, FastMCP 2.x, asyncpg 0.29+, OpenAI SDK 1.x, Pydantic v2

---

## File Map

| File | Responsibility |
|---|---|
| `memory-mcp-server/pyproject.toml` | Package metadata and dependencies |
| `memory-mcp-server/models.py` | `MemoryType` enum (25 types), `SearchFilters` Pydantic model |
| `memory-mcp-server/embeddings.py` | `embed(text) -> list[float]` via OpenAI `text-embedding-ada-002` |
| `memory-mcp-server/db.py` | asyncpg pool lifecycle + 5 query functions |
| `memory-mcp-server/server.py` | FastMCP app, lifespan, 5 `@mcp.tool()` definitions |

---

## Task 1: Scaffold the project

**Files:**
- Create: `memory-mcp-server/pyproject.toml`

- [ ] **Step 1: Create project directory**

```bash
mkdir -p /Users/abishekkumar/Documents/memory-superagents/memory-mcp-server
```

- [ ] **Step 2: Create `memory-mcp-server/pyproject.toml`**

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
```

- [ ] **Step 3: Install dependencies**

```bash
cd /Users/abishekkumar/Documents/memory-superagents/memory-mcp-server
pip install -e .
```

Expected: all 4 packages install without errors. Verify:
```bash
python -c "import fastmcp, asyncpg, openai, pydantic; print('all ok')"
```
Expected output: `all ok`

- [ ] **Step 4: Commit**

```bash
git -C /Users/abishekkumar/Documents/memory-superagents add memory-mcp-server/pyproject.toml
git -C /Users/abishekkumar/Documents/memory-superagents commit -m "feat: scaffold memory-mcp-server"
```

---

## Task 2: models.py — MemoryType enum and SearchFilters

**Files:**
- Create: `memory-mcp-server/models.py`

- [ ] **Step 1: Create `memory-mcp-server/models.py`**

```python
from enum import Enum
from pydantic import BaseModel


class MemoryType(str, Enum):
    preference = "preference"
    profile_fact = "profile_fact"
    project_context = "project_context"
    decision = "decision"
    task = "task"
    event = "event"
    problem = "problem"
    solution = "solution"
    learning = "learning"
    question = "question"
    plan = "plan"
    constraint = "constraint"
    credential_reference = "credential_reference"
    relationship = "relationship"
    routine = "routine"
    artifact = "artifact"
    conversation_summary = "conversation_summary"
    correction = "correction"
    feedback = "feedback"
    observation = "observation"
    hypothesis = "hypothesis"
    experiment = "experiment"
    capability = "capability"
    policy = "policy"
    identity = "identity"


class SearchFilters(BaseModel):
    memory_type: MemoryType | None = None
    scope: str | None = None
    project: str | None = None
```

- [ ] **Step 2: Verify**

```bash
cd /Users/abishekkumar/Documents/memory-superagents/memory-mcp-server
python -c "
from models import MemoryType, SearchFilters
assert MemoryType.decision.value == 'decision'
assert len(MemoryType) == 25
f = SearchFilters(memory_type='decision', project='/tmp/foo')
assert f.memory_type == MemoryType.decision
print('ok')
"
```

Expected output: `ok`

- [ ] **Step 3: Commit**

```bash
git -C /Users/abishekkumar/Documents/memory-superagents add memory-mcp-server/models.py
git -C /Users/abishekkumar/Documents/memory-superagents commit -m "feat: add MemoryType enum and SearchFilters model"
```

---

## Task 3: embeddings.py — OpenAI embed() helper

**Files:**
- Create: `memory-mcp-server/embeddings.py`

- [ ] **Step 1: Create `memory-mcp-server/embeddings.py`**

```python
import os

from openai import AsyncOpenAI

_client: AsyncOpenAI | None = None


def _get_client() -> AsyncOpenAI:
    global _client
    if _client is None:
        api_key = os.environ.get("OPENAI_API_KEY")
        if not api_key:
            raise RuntimeError("OPENAI_API_KEY environment variable is required")
        _client = AsyncOpenAI(api_key=api_key)
    return _client


async def embed(text: str) -> list[float]:
    response = await _get_client().embeddings.create(
        model="text-embedding-ada-002",
        input=text,
    )
    return response.data[0].embedding
```

- [ ] **Step 2: Verify module loads without crashing (client is lazy)**

```bash
cd /Users/abishekkumar/Documents/memory-superagents/memory-mcp-server
python -c "import embeddings; print('loaded ok')"
```

Expected output: `loaded ok` — the `AsyncOpenAI` client is not constructed until `embed()` is first called, so a missing `OPENAI_API_KEY` does not crash at import time.

- [ ] **Step 3: Commit**

```bash
git -C /Users/abishekkumar/Documents/memory-superagents add memory-mcp-server/embeddings.py
git -C /Users/abishekkumar/Documents/memory-superagents commit -m "feat: add OpenAI embed() helper"
```

---

## Task 4: db.py — asyncpg pool and all query functions

**Files:**
- Create: `memory-mcp-server/db.py`

**Key design notes:**
- pgvector type is unknown to asyncpg. Pass embeddings as the string `"[x,y,z,...]"` and use `$n::vector` cast in SQL — PostgreSQL handles the conversion.
- asyncpg auto-decodes `jsonb` columns to Python dicts. No manual JSON parsing needed for reads.
- `id` (UUID) and `created_at` (timestamptz) are non-JSON-serializable. Cast `id` to `text` in SQL; convert `created_at` to ISO string via a helper.
- All writes are orphan (no `session_id`). Project-scoped reads JOIN through `conversation_sessions`.

- [ ] **Step 1: Create `memory-mcp-server/db.py`**

```python
import json
import os
from typing import Any

import asyncpg

_pool: asyncpg.Pool | None = None


async def init_pool() -> None:
    global _pool
    url = os.environ.get("DATABASE_URL")
    if not url:
        raise RuntimeError("DATABASE_URL environment variable is required")
    _pool = await asyncpg.create_pool(url, min_size=1, max_size=5)


async def close_pool() -> None:
    global _pool
    if _pool:
        await _pool.close()
        _pool = None


def _vec(embedding: list[float]) -> str:
    return "[" + ",".join(str(x) for x in embedding) + "]"


def _row(record: asyncpg.Record) -> dict[str, Any]:
    result = {}
    for key, value in record.items():
        if hasattr(value, "isoformat"):
            result[key] = value.isoformat()
        else:
            result[key] = value
    return result


async def search_memories(
    embedding: list[float],
    memory_type: str | None,
    scope: str | None,
    project: str | None,
    limit: int = 10,
) -> list[dict[str, Any]]:
    rows = await _pool.fetch(
        """
        SELECT
            m.id::text,
            m.memory_type,
            m.subject,
            m.content,
            m.importance,
            m.confidence,
            m.scope,
            m.created_at,
            m.metadata,
            1 - (m.embedding <=> $1::vector) AS similarity
        FROM memory_events m
        LEFT JOIN conversation_sessions s ON m.session_id = s.id
        WHERE ($2::text IS NULL OR m.memory_type = $2)
          AND ($3::text IS NULL OR m.scope = $3)
          AND ($4::text IS NULL OR s.workspace_path = $4)
        ORDER BY m.embedding <=> $1::vector
        LIMIT $5
        """,
        _vec(embedding), memory_type, scope, project, limit,
    )
    return [_row(r) for r in rows]


async def write_memory(
    memory_type: str,
    content: str,
    embedding: list[float],
    subject: str | None = None,
    importance: float = 0.5,
    confidence: float = 0.7,
    scope: str = "personal",
    metadata: dict | None = None,
) -> str:
    row = await _pool.fetchrow(
        """
        INSERT INTO memory_events
            (memory_type, subject, content, importance, confidence, scope, metadata, embedding)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::vector)
        RETURNING id::text
        """,
        memory_type,
        subject,
        content,
        importance,
        confidence,
        scope,
        json.dumps(metadata or {}),
        _vec(embedding),
    )
    return row["id"]


async def recent_memories(project: str, limit: int) -> list[dict[str, Any]]:
    rows = await _pool.fetch(
        """
        SELECT
            m.id::text,
            m.memory_type,
            m.subject,
            m.content,
            m.importance,
            m.created_at
        FROM memory_events m
        JOIN conversation_sessions s ON m.session_id = s.id
        WHERE s.workspace_path = $1
        ORDER BY m.created_at DESC
        LIMIT $2
        """,
        project, limit,
    )
    return [_row(r) for r in rows]


async def get_decisions(project: str) -> list[dict[str, Any]]:
    rows = await _pool.fetch(
        """
        SELECT
            m.id::text,
            m.subject,
            m.content,
            m.metadata,
            m.created_at
        FROM memory_events m
        JOIN conversation_sessions s ON m.session_id = s.id
        WHERE s.workspace_path = $1 AND m.memory_type = 'decision'
        ORDER BY m.created_at DESC
        """,
        project,
    )
    return [_row(r) for r in rows]


async def get_preferences_by_embedding(
    embedding: list[float],
    limit: int = 10,
) -> list[dict[str, Any]]:
    rows = await _pool.fetch(
        """
        SELECT
            m.id::text,
            m.subject,
            m.content,
            m.importance,
            m.created_at,
            1 - (m.embedding <=> $1::vector) AS similarity
        FROM memory_events m
        WHERE m.memory_type = 'preference'
        ORDER BY m.embedding <=> $1::vector
        LIMIT $2
        """,
        _vec(embedding), limit,
    )
    return [_row(r) for r in rows]
```

- [ ] **Step 2: Verify module loads**

```bash
cd /Users/abishekkumar/Documents/memory-superagents/memory-mcp-server
python -c "import db; print('loaded ok')"
```

Expected output: `loaded ok`

- [ ] **Step 3: Commit**

```bash
git -C /Users/abishekkumar/Documents/memory-superagents add memory-mcp-server/db.py
git -C /Users/abishekkumar/Documents/memory-superagents commit -m "feat: add asyncpg pool and all query functions"
```

---

## Task 5: server.py — FastMCP app with all 5 tools

**Files:**
- Create: `memory-mcp-server/server.py`

**Notes:**
- `type` is used as a parameter name (shadows Python builtin, but is legal and required for the tool schema to match the spec).
- `metadata` dict is copied before extracting well-known keys (`subject`, `importance`, `confidence`, `scope`) so the caller's dict is not mutated.
- `lifespan` validates env vars and initializes the DB pool at startup; tears down the pool on shutdown.

- [ ] **Step 1: Create `memory-mcp-server/server.py`**

```python
import os
from contextlib import asynccontextmanager
from typing import Any

from fastmcp import FastMCP

import db
import embeddings
from models import MemoryType, SearchFilters


def _check_env() -> None:
    missing = [v for v in ("DATABASE_URL", "OPENAI_API_KEY") if not os.environ.get(v)]
    if missing:
        raise RuntimeError(
            f"Missing required environment variables: {', '.join(missing)}"
        )


@asynccontextmanager
async def lifespan(app: FastMCP):
    _check_env()
    await db.init_pool()
    yield
    await db.close_pool()


mcp = FastMCP("memory", lifespan=lifespan)


@mcp.tool(name="memory.search")
async def memory_search(
    query: str,
    filters: SearchFilters | None = None,
) -> list[dict[str, Any]]:
    """Semantic search across all memory events. Returns top 10 by cosine similarity."""
    f = filters or SearchFilters()
    embedding = await embeddings.embed(query)
    return await db.search_memories(
        embedding=embedding,
        memory_type=f.memory_type.value if f.memory_type else None,
        scope=f.scope,
        project=f.project,
    )


@mcp.tool(name="memory.write")
async def memory_write(
    type: MemoryType,
    content: str,
    metadata: dict[str, Any] | None = None,
) -> dict[str, str]:
    """Write a new memory event. Well-known metadata keys: subject, importance, confidence, scope."""
    meta = dict(metadata or {})
    subject = meta.pop("subject", None)
    importance = float(meta.pop("importance", 0.5))
    confidence = float(meta.pop("confidence", 0.7))
    scope = str(meta.pop("scope", "personal"))

    embedding = await embeddings.embed(content)
    memory_id = await db.write_memory(
        memory_type=type.value,
        content=content,
        embedding=embedding,
        subject=subject,
        importance=importance,
        confidence=confidence,
        scope=scope,
        metadata=meta,
    )
    return {"id": memory_id}


@mcp.tool(name="memory.recent")
async def memory_recent(
    project: str,
    limit: int = 20,
) -> list[dict[str, Any]]:
    """Fetch the most recent memory events for a project (workspace_path), newest first."""
    return await db.recent_memories(project=project, limit=limit)


@mcp.tool(name="memory.get_decisions")
async def memory_get_decisions(
    project: str,
) -> list[dict[str, Any]]:
    """Fetch all decisions recorded for a project (workspace_path), newest first."""
    return await db.get_decisions(project=project)


@mcp.tool(name="memory.get_preferences")
async def memory_get_preferences(
    topic: str,
) -> list[dict[str, Any]]:
    """Semantic search over preferences. Returns top 10 most relevant to the topic."""
    embedding = await embeddings.embed(topic)
    return await db.get_preferences_by_embedding(embedding=embedding)


if __name__ == "__main__":
    mcp.run()
```

- [ ] **Step 2: Verify server.py imports cleanly**

```bash
cd /Users/abishekkumar/Documents/memory-superagents/memory-mcp-server
python -c "import server; print('loaded ok')"
```

Expected output: `loaded ok` — lifespan only runs during `mcp.run()`, so missing env vars do not crash at import time.

- [ ] **Step 3: Commit**

```bash
git -C /Users/abishekkumar/Documents/memory-superagents add memory-mcp-server/server.py
git -C /Users/abishekkumar/Documents/memory-superagents commit -m "feat: add FastMCP server with all 5 memory tools"
```

---

## Task 6: Apply schema, register, and smoke test

- [ ] **Step 1: Apply schema to PostgreSQL**

```bash
psql $DATABASE_URL -f /Users/abishekkumar/Documents/memory-superagents/schema.sql
```

Expected: output includes `CREATE EXTENSION`, `CREATE TABLE` × 3, no errors.

If running for the first time the extension may warn `extension "vector" already exists` — that is fine.

- [ ] **Step 2: Register the MCP server with Claude Code**

```bash
claude mcp add memory \
  -e DATABASE_URL="<your-postgres-connection-string>" \
  -e OPENAI_API_KEY="<your-openai-key>" \
  -- python /Users/abishekkumar/Documents/memory-superagents/memory-mcp-server/server.py
```

Expected: `Added MCP server memory`

- [ ] **Step 3: Confirm server appears in the MCP list**

```bash
claude mcp list
```

Expected: `memory` is listed with its 5 tools visible.

- [ ] **Step 4: Smoke test — write then retrieve**

In a Claude Code session, call:

1. `memory.write` with:
   - `type`: `"preference"`
   - `content`: `"I prefer dark mode in all editors and terminals"`
   - `metadata`: `{"subject": "editor theme preference", "importance": 0.8}`

2. `memory.get_preferences` with:
   - `topic`: `"editor theme"`

Expected: the written preference appears in the top results with a high similarity score (> 0.85).

3. `memory.search` with:
   - `query`: `"dark mode"`
   - `filters`: `{"memory_type": "preference"}`

Expected: same record returned.
