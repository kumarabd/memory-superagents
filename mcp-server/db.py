import json
import os
from typing import Any

import asyncpg

_pool: asyncpg.Pool | None = None

# Excludes superseded/stale memories from all read queries by default.
# Applied as: AND (<_ACTIVE_FILTER>)
_ACTIVE_FILTER = "(m.metadata->>'status' IS NULL OR m.metadata->>'status' = 'active')"


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
        WHERE (
            (m.session_id IS NULL AND m.metadata->>'project' = $1)
            OR (m.session_id IS NOT NULL AND s.workspace_path = $1)
        )
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
        WHERE (
            (m.session_id IS NULL AND m.metadata->>'project' = $1)
            OR (m.session_id IS NOT NULL AND s.workspace_path = $1)
        )
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
        WHERE (
            (m.session_id IS NULL AND m.metadata->>'project' = $1)
            OR (m.session_id IS NOT NULL AND s.workspace_path = $1)
        )
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
