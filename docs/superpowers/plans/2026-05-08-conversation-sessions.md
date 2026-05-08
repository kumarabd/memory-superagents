# Conversation Sessions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Populate `conversation_sessions` on MCP server startup and link every `memory_event` to its originating Claude Code session via `session_id`.

**Architecture:** On lifespan start, read `CLAUDE_CODE_SESSION_ID` from env and INSERT a `conversation_sessions` row using that UUID as the primary key — the same UUID Claude Code uses to name the JSONL transcript file. A module-level `_active_session_id` in `db.py` is set at startup and read by `write_memory` to auto-stamp every new `memory_event`. On lifespan exit, `ended_at` is set. If `CLAUDE_CODE_SESSION_ID` is absent, the server starts normally with `_active_session_id = None` and `session_id` stays NULL (existing behaviour preserved).

**Tech Stack:** Python 3.11+, asyncpg, pytest, pytest-asyncio, unittest.mock

---

## File Map

| File | Change |
|------|--------|
| `mcp-server/requirements.txt` | Add `pytest>=8.0` and `pytest-asyncio>=0.23` |
| `mcp-server/pyproject.toml` | Add dev-dependencies and pytest asyncio config |
| `mcp-server/common/db.py` | Add `_active_session_id`, `create_session()`, `close_session()`; patch `write_memory()` |
| `mcp-server/server.py` | Call `db.create_session()` / `db.close_session()` in lifespan |
| `mcp-server/tests/__init__.py` | New empty file (makes `tests/` a package) |
| `mcp-server/tests/test_db_session.py` | New test file for session functions and write_memory stamping |

---

## Task 1: Add test dependencies

**Files:**
- Modify: `mcp-server/requirements.txt`
- Modify: `mcp-server/pyproject.toml`

- [ ] **Step 1: Add pytest deps to requirements.txt**

Replace the contents of `mcp-server/requirements.txt` with:

```
# Install into mcp-server/.venv by run-mcp-server.sh (no global uv required).
fastmcp>=2.0
asyncpg>=0.29
openai>=1.0
pydantic>=2.0
pytest>=8.0
pytest-asyncio>=0.23
```

- [ ] **Step 2: Add pytest asyncio mode to pyproject.toml**

Add this section at the end of `mcp-server/pyproject.toml`:

```toml
[tool.pytest.ini_options]
asyncio_mode = "auto"
testpaths = ["tests"]
```

- [ ] **Step 3: Install into the venv**

```bash
cd mcp-server && .venv/bin/pip install pytest pytest-asyncio
```

Expected: both packages install without error.

- [ ] **Step 4: Verify pytest runs**

```bash
cd mcp-server && .venv/bin/pytest --collect-only
```

Expected: `no tests ran` (0 items collected) — confirms pytest is wired up.

- [ ] **Step 5: Commit**

```bash
git add mcp-server/requirements.txt mcp-server/pyproject.toml
git commit -m "chore: add pytest and pytest-asyncio to mcp-server dev deps"
```

---

## Task 2: Write failing tests for session functions

**Files:**
- Create: `mcp-server/tests/__init__.py`
- Create: `mcp-server/tests/test_db_session.py`

- [ ] **Step 1: Create the tests package**

Create `mcp-server/tests/__init__.py` as an empty file.

- [ ] **Step 2: Write failing tests**

Create `mcp-server/tests/test_db_session.py`:

```python
import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import common.db as db


@pytest.fixture(autouse=True)
def reset_session_state():
    """Reset module-level session state between tests."""
    db._active_session_id = None
    yield
    db._active_session_id = None


def _make_pool(fetchrow_return=None, execute_return=None):
    pool = MagicMock()
    pool.fetchrow = AsyncMock(return_value=fetchrow_return)
    pool.execute = AsyncMock(return_value=execute_return)
    return pool


# --- create_session ---

async def test_create_session_sets_active_session_id():
    session_id = str(uuid.uuid4())
    pool = _make_pool()
    with patch.object(db, "_pool", pool):
        await db.create_session(session_id=session_id, workspace_path="/tmp/project")
    assert db._active_session_id == session_id


async def test_create_session_inserts_row():
    session_id = str(uuid.uuid4())
    pool = _make_pool()
    with patch.object(db, "_pool", pool):
        await db.create_session(session_id=session_id, workspace_path="/tmp/project")
    pool.execute.assert_awaited_once()
    call_args = pool.execute.call_args
    sql = call_args[0][0]
    assert "INSERT INTO conversation_sessions" in sql
    # session_id, workspace_path, agent_name passed as positional args
    args = call_args[0][1:]
    assert session_id in args
    assert "/tmp/project" in args
    assert "claude-code" in args


async def test_create_session_skips_when_no_session_id():
    pool = _make_pool()
    with patch.object(db, "_pool", pool):
        await db.create_session(session_id=None, workspace_path="/tmp/project")
    pool.execute.assert_not_awaited()
    assert db._active_session_id is None


# --- close_session ---

async def test_close_session_sets_ended_at():
    session_id = str(uuid.uuid4())
    pool = _make_pool()
    db._active_session_id = session_id
    with patch.object(db, "_pool", pool):
        await db.close_session()
    pool.execute.assert_awaited_once()
    call_args = pool.execute.call_args
    sql = call_args[0][0]
    assert "UPDATE conversation_sessions" in sql
    assert "ended_at" in sql
    args = call_args[0][1:]
    assert session_id in args


async def test_close_session_clears_active_session_id():
    session_id = str(uuid.uuid4())
    pool = _make_pool()
    db._active_session_id = session_id
    with patch.object(db, "_pool", pool):
        await db.close_session()
    assert db._active_session_id is None


async def test_close_session_noop_when_no_active_session():
    pool = _make_pool()
    db._active_session_id = None
    with patch.object(db, "_pool", pool):
        await db.close_session()
    pool.execute.assert_not_awaited()


# --- write_memory session stamping ---

async def test_write_memory_stamps_active_session_id():
    session_id = str(uuid.uuid4())
    memory_id = str(uuid.uuid4())
    pool = _make_pool(fetchrow_return={"id": memory_id})
    db._active_session_id = session_id
    with patch.object(db, "_pool", pool):
        result = await db.write_memory(
            memory_type="decision",
            content="use approach A",
            embedding=[0.1] * 1536,
        )
    assert result == memory_id
    call_args = pool.fetchrow.call_args
    sql = call_args[0][0]
    assert "session_id" in sql
    args = call_args[0][1:]
    assert session_id in args


async def test_write_memory_null_session_when_no_active_session():
    memory_id = str(uuid.uuid4())
    pool = _make_pool(fetchrow_return={"id": memory_id})
    db._active_session_id = None
    with patch.object(db, "_pool", pool):
        await db.write_memory(
            memory_type="preference",
            content="terse responses",
            embedding=[0.1] * 1536,
        )
    call_args = pool.fetchrow.call_args
    args = call_args[0][1:]
    # None should appear as the session_id arg (last positional after embedding)
    assert None in args
```

- [ ] **Step 3: Run tests — verify they all fail**

```bash
cd mcp-server && .venv/bin/pytest tests/test_db_session.py -v
```

Expected: all 9 tests FAIL with `AttributeError: module 'common.db' has no attribute '_active_session_id'` or similar — confirms the tests are wired correctly and the functions don't exist yet.

- [ ] **Step 4: Commit failing tests**

```bash
git add mcp-server/tests/
git commit -m "test: add failing tests for session create/close and write_memory stamping"
```

---

## Task 3: Implement `_active_session_id`, `create_session`, `close_session` in db.py

**Files:**
- Modify: `mcp-server/common/db.py`

- [ ] **Step 1: Add `_active_session_id` and session functions**

In `mcp-server/common/db.py`, add the following directly after the `_pool` declaration block (after line 11, before `PROJECT_MATCH`):

```python
_active_session_id: str | None = None


async def create_session(session_id: str | None, workspace_path: str) -> None:
    global _active_session_id
    if not session_id:
        return
    await _get_pool().execute(
        """
        INSERT INTO conversation_sessions (id, workspace_path, agent_name)
        VALUES ($1::uuid, $2, $3)
        ON CONFLICT (id) DO NOTHING
        """,
        session_id,
        workspace_path,
        "claude-code",
    )
    _active_session_id = session_id


async def close_session() -> None:
    global _active_session_id
    if not _active_session_id:
        return
    await _get_pool().execute(
        """
        UPDATE conversation_sessions
        SET ended_at = now()
        WHERE id = $1::uuid
        """,
        _active_session_id,
    )
    _active_session_id = None
```

- [ ] **Step 2: Run session lifecycle tests — verify they pass**

```bash
cd mcp-server && .venv/bin/pytest tests/test_db_session.py -v -k "session"
```

Expected: 6 session tests PASS, 3 write_memory tests still FAIL.

---

## Task 4: Patch `write_memory` to auto-stamp `session_id`

**Files:**
- Modify: `mcp-server/common/db.py` (the `write_memory` function)

- [ ] **Step 1: Update `write_memory` to include `session_id`**

Replace the `write_memory` function in `mcp-server/common/db.py`:

```python
async def write_memory(
    memory_type: str,
    content: str,
    embedding: list[float],
    subject: str | None = None,
    importance: float = 0.5,
    confidence: float = 0.7,
    scope: str = "personal",
    metadata: dict[str, Any] | None = None,
) -> str:
    row = await _get_pool().fetchrow(
        """
        INSERT INTO memory_events
            (memory_type, subject, content, importance, confidence, scope, metadata, embedding, session_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::vector, $9::uuid)
        RETURNING id::text
        """,
        memory_type,
        subject,
        content,
        importance,
        confidence,
        scope,
        metadata or {},
        _vec(embedding),
        _active_session_id,
    )
    return row["id"]
```

- [ ] **Step 2: Run all tests — verify all 9 pass**

```bash
cd mcp-server && .venv/bin/pytest tests/test_db_session.py -v
```

Expected: all 9 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add mcp-server/common/db.py
git commit -m "feat: add session lifecycle functions and auto-stamp session_id on memory writes"
```

---

## Task 5: Wire session lifecycle into server.py lifespan

**Files:**
- Modify: `mcp-server/server.py`

- [ ] **Step 1: Update lifespan to call create_session and close_session**

Replace the `lifespan` function in `mcp-server/server.py`:

```python
@asynccontextmanager
async def lifespan(app: FastMCP):
    _check_env()
    await db.init_pool()
    await db.create_session(
        session_id=os.environ.get("CLAUDE_CODE_SESSION_ID"),
        workspace_path=os.getcwd(),
    )
    yield
    await db.close_session()
    await db.close_pool()
    await embeddings.close()
```

- [ ] **Step 2: Commit**

```bash
git add mcp-server/server.py
git commit -m "feat: wire session create/close into MCP server lifespan"
```

---

## Task 6: Verify end-to-end

- [ ] **Step 1: Confirm `CLAUDE_CODE_SESSION_ID` is set in the current environment**

```bash
echo $CLAUDE_CODE_SESSION_ID
```

Expected: a UUID like `9d540854-e7fa-41d1-b9e5-50b2d995eb18`.

- [ ] **Step 2: Restart the MCP server**

In Claude Code, run `/mcp` to confirm the memory server is connected, then disconnect and reconnect (or restart Claude Code) so the new lifespan code runs.

- [ ] **Step 3: Verify the session row was inserted**

```sql
SELECT id, workspace_path, agent_name, started_at, ended_at
FROM conversation_sessions
ORDER BY started_at DESC
LIMIT 3;
```

Expected: a row with `id` matching `$CLAUDE_CODE_SESSION_ID`, `agent_name = 'claude-code'`, `ended_at = NULL`.

- [ ] **Step 4: Write a test memory via MCP tool, then verify session linkage**

Ask Claude to call `memory.write` with any content (e.g. "test memory for session verification"). Then:

```sql
SELECT m.id, m.memory_type, m.content, m.session_id, s.workspace_path
FROM memory_events m
JOIN conversation_sessions s ON m.session_id = s.id
ORDER BY m.created_at DESC
LIMIT 1;
```

Expected: the new memory row has `session_id` matching the session UUID, and `workspace_path` matching the project directory.

- [ ] **Step 5: Confirm transcript linkage**

```bash
ls ~/.claude/projects/-Users-abishekkumar-Documents-memory-superagents/ | grep <session_id_from_above>
```

Expected: the JSONL file exists with that exact UUID as filename.
