"""Register `memory.*` tools on a FastMCP instance."""

from typing import Any

from fastmcp import FastMCP

from common import db, embeddings
from common.models import MemoryType, SearchFilters


def register_core_tools(mcp: FastMCP) -> None:
    @mcp.tool(name="memory.search")
    async def memory_search(
        query: str,
        filters: SearchFilters | None = None,
        limit: int = 10,
        include_inactive: bool = False,
    ) -> list[dict[str, Any]]:
        """Semantic search across memory events. Excludes superseded/stale by default."""
        f = filters or SearchFilters()
        embedding = await embeddings.embed(query)
        return await db.search_memories(
            embedding=embedding,
            memory_type=f.memory_type.value if f.memory_type else None,
            scope=f.scope,
            project=f.project,
            limit=max(1, min(limit, 200)),
            include_inactive=include_inactive,
        )

    @mcp.tool(name="memory.write")
    async def memory_write(
        type: MemoryType,
        content: str,
        metadata: dict[str, Any] | None = None,
    ) -> dict[str, str]:
        """Write a memory event.

        Column-level keys (extracted from metadata if present):
          subject, importance, confidence, scope

        Metadata keys stored verbatim in JSONB:
          project, workspace_path, topic, tags, source,
          session_id, created_from, status, and any others
        """
        meta = dict(metadata or {})
        subject = meta.pop("subject", None)
        try:
            importance = float(meta.pop("importance", 0.5))
            confidence = float(meta.pop("confidence", 0.7))
        except (TypeError, ValueError) as e:
            raise ValueError(f"importance and confidence must be numeric: {e}") from e
        scope = str(meta.pop("scope", "personal"))

        if "status" not in meta:
            meta["status"] = "active"

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
        include_inactive: bool = False,
    ) -> list[dict[str, Any]]:
        """Fetch the most recent active memory events for a project, newest first."""
        return await db.recent_memories(
            project=project,
            limit=max(1, min(limit, 200)),
            include_inactive=include_inactive,
        )

    @mcp.tool(name="memory.get_decisions")
    async def memory_get_decisions(
        project: str,
        limit: int = 100,
        include_inactive: bool = False,
    ) -> list[dict[str, Any]]:
        """Fetch active decisions for a project (workspace_path), newest first."""
        return await db.get_decisions(
            project=project,
            limit=max(1, min(limit, 500)),
            include_inactive=include_inactive,
        )

    @mcp.tool(name="memory.get_project_context")
    async def memory_get_project_context(
        project: str,
        limit: int = 10,
        include_inactive: bool = False,
    ) -> list[dict[str, Any]]:
        """Fetch project_context memories for a workspace, newest first."""
        return await db.get_project_context(
            project=project,
            limit=max(1, min(limit, 50)),
            include_inactive=include_inactive,
        )

    @mcp.tool(name="memory.close_session")
    async def memory_close_session(summary: str) -> dict[str, str]:
        """Persist a session summary and mark the session ended.

        Call this once at the end of every session with a concise summary of
        what was discussed, decisions made, problems solved, and next steps.
        The text is stored in conversation_sessions.summary and linked to all
        memory_events written during this session.
        """
        await db.close_session(summary=summary)
        return {"status": "ok"}

    @mcp.tool(name="memory.get_preferences")
    async def memory_get_preferences(
        topic: str,
        limit: int = 10,
        include_inactive: bool = False,
    ) -> list[dict[str, Any]]:
        """Semantic search over active preferences, most relevant first."""
        embedding = await embeddings.embed(topic)
        return await db.get_preferences_by_embedding(
            embedding=embedding,
            limit=max(1, min(limit, 200)),
            include_inactive=include_inactive,
        )
