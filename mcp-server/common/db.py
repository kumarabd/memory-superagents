import json
import os
import sys
import uuid
from typing import Any

import asyncpg

_pool: asyncpg.Pool | None = None

# Excludes superseded/stale memories from all read queries by default.
# Applied as: AND (<_ACTIVE_FILTER>)
_ACTIVE_FILTER = "(m.metadata->>'status' IS NULL OR m.metadata->>'status' = 'active')"


_active_session_id: str | None = None


async def create_session(session_id: str | None, workspace_path: str) -> None:
    global _active_session_id
    # Claude Code does not reliably provide a stable session id to stdio MCP servers.
    # We still create a DB session row so memory events can be stamped consistently
    # for the lifetime of this MCP server process.
    if not session_id:
        session_id = str(uuid.uuid4())
        print(
            "WARN: create_session called without a session_id — "
            "CLAUDE_CODE_SESSION_ID is not set in the MCP server environment. "
            f"Generated session id: {session_id}",
            file=sys.stderr,
        )
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


async def close_session(summary: str | None = None) -> None:
    global _active_session_id
    if not _active_session_id:
        return
    await _get_pool().execute(
        """
        UPDATE conversation_sessions
        SET ended_at = now(), summary = COALESCE($2, summary)
        WHERE id = $1::uuid
        """,
        _active_session_id,
        summary,
    )
    _active_session_id = None


PROJECT_MATCH = """(
            (m.session_id IS NULL AND m.metadata->>'project' = $1)
            OR (m.session_id IS NOT NULL AND s.workspace_path = $1)
        )"""


async def _setup_codecs(conn: asyncpg.Connection) -> None:
    await conn.set_type_codec('jsonb', encoder=json.dumps, decoder=json.loads, schema='pg_catalog')
    await conn.set_type_codec('json', encoder=json.dumps, decoder=json.loads, schema='pg_catalog')


async def init_pool() -> None:
    global _pool
    url = os.environ.get("DATABASE_URL")
    if not url:
        raise RuntimeError("DATABASE_URL environment variable is required")
    _pool = await asyncpg.create_pool(url, min_size=1, max_size=5, init=_setup_codecs)


async def close_pool() -> None:
    global _pool
    if _pool:
        await _pool.close()
        _pool = None


def _get_pool() -> asyncpg.Pool:
    if _pool is None:
        raise RuntimeError("Database pool is not initialised — call init_pool() first")
    return _pool


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
    include_inactive: bool = False,
) -> list[dict[str, Any]]:
    active_clause = "" if include_inactive else f"AND {_ACTIVE_FILTER}"
    rows = await _get_pool().fetch(
        f"""
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
          AND (
            $4::text IS NULL
            OR s.workspace_path = $4
            OR m.metadata->>'project' = $4
          )
          {active_clause}
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


async def recent_memories(
    project: str,
    limit: int,
    include_inactive: bool = False,
) -> list[dict[str, Any]]:
    active_clause = "" if include_inactive else f"AND {_ACTIVE_FILTER}"
    rows = await _get_pool().fetch(
        f"""
        SELECT
            m.id::text,
            m.memory_type,
            m.subject,
            m.content,
            m.importance,
            m.created_at
        FROM memory_events m
        LEFT JOIN conversation_sessions s ON m.session_id = s.id
        WHERE {PROJECT_MATCH}
          {active_clause}
        ORDER BY m.created_at DESC
        LIMIT $2
        """,
        project, limit,
    )
    return [_row(r) for r in rows]


async def get_decisions(
    project: str,
    limit: int = 100,
    include_inactive: bool = False,
) -> list[dict[str, Any]]:
    active_clause = "" if include_inactive else f"AND {_ACTIVE_FILTER}"
    rows = await _get_pool().fetch(
        f"""
        SELECT
            m.id::text,
            m.subject,
            m.content,
            m.importance,
            m.confidence,
            m.metadata,
            m.created_at
        FROM memory_events m
        LEFT JOIN conversation_sessions s ON m.session_id = s.id
        WHERE {PROJECT_MATCH}
          AND m.memory_type = 'decision'
          {active_clause}
        ORDER BY m.created_at DESC
        LIMIT $2
        """,
        project, limit,
    )
    return [_row(r) for r in rows]


async def get_project_context(
    project: str,
    limit: int = 10,
    include_inactive: bool = False,
) -> list[dict[str, Any]]:
    active_clause = "" if include_inactive else f"AND {_ACTIVE_FILTER}"
    rows = await _get_pool().fetch(
        f"""
        SELECT
            m.id::text,
            m.subject,
            m.content,
            m.importance,
            m.confidence,
            m.metadata,
            m.created_at
        FROM memory_events m
        LEFT JOIN conversation_sessions s ON m.session_id = s.id
        WHERE {PROJECT_MATCH}
          AND m.memory_type = 'project_context'
          {active_clause}
        ORDER BY m.created_at DESC
        LIMIT $2
        """,
        project, limit,
    )
    return [_row(r) for r in rows]


async def get_preferences_by_embedding(
    embedding: list[float],
    limit: int = 10,
    include_inactive: bool = False,
) -> list[dict[str, Any]]:
    active_clause = "" if include_inactive else f"AND {_ACTIVE_FILTER}"
    rows = await _get_pool().fetch(
        f"""
        SELECT
            m.id::text,
            m.subject,
            m.content,
            m.importance,
            m.confidence,
            m.created_at,
            1 - (m.embedding <=> $1::vector) AS similarity
        FROM memory_events m
        WHERE m.memory_type = 'preference'
          {active_clause}
        ORDER BY m.embedding <=> $1::vector
        LIMIT $2
        """,
        _vec(embedding), limit,
    )
    return [_row(r) for r in rows]


async def project_memory_distribution(
    project: str,
    include_inactive: bool = False,
) -> dict[str, Any]:
    """Counts by memory_type for a workspace + total span of created_at."""
    active_clause = "" if include_inactive else f"AND {_ACTIVE_FILTER}"
    pool = _get_pool()
    type_rows = await pool.fetch(
        f"""
        SELECT m.memory_type, COUNT(*)::int AS count
        FROM memory_events m
        LEFT JOIN conversation_sessions s ON m.session_id = s.id
        WHERE {PROJECT_MATCH}
          {active_clause}
        GROUP BY m.memory_type
        ORDER BY count DESC
        """,
        project,
    )
    agg = await pool.fetchrow(
        f"""
        SELECT
            COUNT(*)::int AS total,
            MIN(m.created_at) AS first_event_at,
            MAX(m.created_at) AS last_event_at
        FROM memory_events m
        LEFT JOIN conversation_sessions s ON m.session_id = s.id
        WHERE {PROJECT_MATCH}
          {active_clause}
        """,
        project,
    )
    by_type = [{"memory_type": r["memory_type"], "count": r["count"]} for r in type_rows]
    return {
        "project": project,
        "total": agg["total"] if agg else 0,
        "first_event_at": agg["first_event_at"].isoformat() if agg and agg["first_event_at"] else None,
        "last_event_at": agg["last_event_at"].isoformat() if agg and agg["last_event_at"] else None,
        "by_type": by_type,
    }


# --- AgentLab notebook (structured JSON in agentlab_notebook) ---

NOTEBOOK_KEYS = frozenset(
    {
        "term_cache",
        "concept_mapping",
        "preferences",
        "hypotheses",
        "experiments",
        "findings",
        "semantic_links",
        "open_questions",
        "artifacts",
    }
)


def default_notebook_payload() -> dict[str, Any]:
    return {
        "term_cache": {},
        "concept_mapping": {},
        "preferences": {"row_cap": 100},
        "hypotheses": [],
        "experiments": [],
        "findings": [],
        "semantic_links": [],
        "open_questions": [],
        "artifacts": [],
    }


async def notebook_load(workspace_key: str) -> dict[str, Any]:
    pool = _get_pool()
    if not pool:
        return {
            "workspace_key": workspace_key,
            "version": 0,
            "updated_at": None,
            "notebook": default_notebook_payload(),
        }
    try:
        row = await pool.fetchrow(
            """
            SELECT payload, version, updated_at
            FROM agentlab_notebook
            WHERE workspace_key = $1
            """,
            workspace_key,
        )
        if row is None:
            # Materialize default row on first touch so the table is visible in ops
            # (notebook.patch also inserts; load alone used to leave zero rows).
            await pool.execute(
                """
                INSERT INTO agentlab_notebook (workspace_key, payload, version, updated_at)
                VALUES ($1, $2::jsonb, 1, now())
                ON CONFLICT (workspace_key) DO NOTHING
                """,
                workspace_key,
                json.dumps(default_notebook_payload()),
            )
            row = await pool.fetchrow(
                """
                SELECT payload, version, updated_at
                FROM agentlab_notebook
                WHERE workspace_key = $1
                """,
                workspace_key,
            )
        if row is None:
            return {
                "workspace_key": workspace_key,
                "version": 0,
                "updated_at": None,
                "notebook": default_notebook_payload(),
            }
        payload = row["payload"]
        if not isinstance(payload, dict):
            payload = default_notebook_payload()
        else:
            merged = default_notebook_payload()
            merged.update(payload)
            payload = merged
        return {
            "workspace_key": workspace_key,
            "version": int(row["version"]),
            "updated_at": row["updated_at"].isoformat() if row["updated_at"] else None,
            "notebook": payload,
        }
    except Exception:
        return {
            "workspace_key": workspace_key,
            "version": 0,
            "updated_at": None,
            "notebook": default_notebook_payload(),
            "_proxy": {
                "configured": bool(pool),
                "error": "agentlab_notebook missing or DB error — run `memory migrate`",
            },
        }


async def notebook_patch(
    workspace_key: str,
    patch: dict[str, Any],
    expected_version: int | None,
) -> dict[str, Any]:
    pool = _get_pool()
    bad_keys = [k for k in patch if k not in NOTEBOOK_KEYS]
    if bad_keys:
        raise ValueError(
            f"notebook.patch: unknown keys {bad_keys!r}; allowed: {sorted(NOTEBOOK_KEYS)}"
        )

    async with pool.acquire() as conn:
        async with conn.transaction():
            row = await conn.fetchrow(
                """
                SELECT payload, version
                FROM agentlab_notebook
                WHERE workspace_key = $1
                FOR UPDATE
                """,
                workspace_key,
            )
            if row is None:
                if expected_version is not None and expected_version != 0:
                    raise ValueError(
                        f"notebook.patch: no row for workspace; expected_version "
                        f"must be 0 or None, got {expected_version}"
                    )
                base = default_notebook_payload()
                cur_v = 0
            else:
                cur_v = int(row["version"])
                if expected_version is not None and cur_v != expected_version:
                    raise ValueError(
                        f"notebook.patch: version mismatch (current={cur_v}, "
                        f"expected={expected_version})"
                    )
                raw = row["payload"]
                base = default_notebook_payload()
                if isinstance(raw, dict):
                    base.update(raw)

            for k in NOTEBOOK_KEYS:
                if k in patch:
                    base[k] = patch[k]

            new_v = cur_v + 1
            await conn.execute(
                """
                INSERT INTO agentlab_notebook (workspace_key, payload, version, updated_at)
                VALUES ($1, $2::jsonb, $3, now())
                ON CONFLICT (workspace_key) DO UPDATE
                SET payload = EXCLUDED.payload,
                    version = EXCLUDED.version,
                    updated_at = now()
                """,
                workspace_key,
                json.dumps(base),
                new_v,
            )

    return {
        "workspace_key": workspace_key,
        "version": new_v,
        "ok": True,
    }
