# Conversation Sessions — Design Spec

**Date:** 2026-05-08
**Status:** Approved

---

## Problem

`memory_events` rows are written with `session_id = NULL`. Every memory floats free — no provenance, no way to trace which conversation produced it, no link to the full Claude Code transcript that contains the context behind the decision.

---

## Goal

Populate `conversation_sessions` so that every `memory_event` row carries a `session_id` pointing to the Claude Code conversation that produced it. Given a memory, you can always find the full JSONL transcript and read the original context.

The `messages` table is explicitly out of scope — Claude Code already stores complete transcripts at `~/.claude/projects/<project>/<session_id>.jsonl`. Duplicating them in Postgres adds no value.

---

## Key Insight: Session ID Is Already Known

Claude Code sets `CLAUDE_CODE_SESSION_ID` as an environment variable in every process it spawns — including the MCP server subprocess. This UUID matches the JSONL filename exactly:

```
~/.claude/projects/-Users-abishekkumar-Documents-memory-superagents/
  9d540854-e7fa-41d1-b9e5-50b2d995eb18.jsonl   ← transcript
```

```sql
SELECT id FROM conversation_sessions;
-- 9d540854-e7fa-41d1-b9e5-50b2d995eb18        ← same UUID
```

No UUID generation needed. The link between Postgres and the filesystem is free.

---

## Architecture

### Session Lifecycle (server.py — lifespan hook)

```
MCP server starts
  └─ read CLAUDE_CODE_SESSION_ID from env
  └─ INSERT into conversation_sessions (id, workspace_path, agent_name, started_at)
  └─ store session_id in db._active_session_id

MCP server shuts down
  └─ UPDATE conversation_sessions SET ended_at = now() WHERE id = _active_session_id
```

### Auto-stamping memory_events (db.py — write_memory)

```
memory.write called
  └─ write_memory() reads db._active_session_id
  └─ passes it as session_id column in INSERT
```

No changes to the `memory.write` MCP tool interface. The AI continues calling it exactly as before — stamping is invisible.

### Graceful degradation

If `CLAUDE_CODE_SESSION_ID` is absent (e.g. running the server outside Claude Code for testing), skip the INSERT and leave `_active_session_id = None`. `write_memory` falls back to `session_id = NULL` — existing behaviour, no crash.

---

## Data Written

**On startup — INSERT:**

| Column         | Value                                      |
|----------------|--------------------------------------------|
| `id`           | `CLAUDE_CODE_SESSION_ID` env var (UUID)    |
| `workspace_path` | `os.getcwd()` at startup                 |
| `agent_name`   | `"claude-code"`                            |
| `started_at`   | `now()` (Postgres default)                 |
| `ended_at`     | NULL                                       |
| `summary`      | NULL (future use)                          |
| `metadata`     | `{}` (future use)                          |

**On shutdown — UPDATE:**

| Column     | Value   |
|------------|---------|
| `ended_at` | `now()` |

---

## Files Changed

| File | Change |
|------|--------|
| `mcp-server/common/db.py` | Add `_active_session_id` module var; add `create_session()` and `close_session()` functions; patch `write_memory()` to pass `session_id` |
| `mcp-server/server.py` | Call `db.create_session()` at lifespan start; call `db.close_session()` at lifespan exit |

No new files. No migration needed (the `id` column already accepts an explicit UUID — the `DEFAULT gen_random_uuid()` only applies when id is omitted).

---

## What You Can Do After This

- **Provenance query:** Given a memory, find its conversation.
  ```sql
  SELECT s.workspace_path, s.started_at, s.ended_at
  FROM memory_events m
  JOIN conversation_sessions s ON m.session_id = s.id
  WHERE m.id = '<memory-uuid>';
  ```

- **Transcript lookup:** The `session_id` UUID is the JSONL filename.
  ```
  ~/.claude/projects/<encoded-workspace>/<session_id>.jsonl
  ```

- **Session memory list:** All memories produced in a given conversation.
  ```sql
  SELECT memory_type, subject, content
  FROM memory_events
  WHERE session_id = '<session-uuid>'
  ORDER BY created_at;
  ```

---

## Explicitly Out of Scope

- `messages` table — not populated; transcripts already exist as JSONL
- `summary` field on sessions — left NULL; future use
- Multi-agent or concurrent session support — one session per server process
