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
    await embeddings.close()


mcp = FastMCP("memory", lifespan=lifespan)


@mcp.tool(name="memory.search")
async def memory_search(
    query: str,
    filters: SearchFilters | None = None,
    limit: int = 10,
) -> list[dict[str, Any]]:
    """Semantic search across all memory events."""
    f = filters or SearchFilters()
    embedding = await embeddings.embed(query)
    return await db.search_memories(
        embedding=embedding,
        memory_type=f.memory_type.value if f.memory_type else None,
        scope=f.scope,
        project=f.project,
        limit=max(1, min(limit, 200)),
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
    try:
        importance = float(meta.pop("importance", 0.5))
        confidence = float(meta.pop("confidence", 0.7))
    except (TypeError, ValueError) as e:
        raise ValueError(f"importance and confidence must be numeric: {e}") from e
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
    return await db.recent_memories(project=project, limit=max(1, min(limit, 200)))


@mcp.tool(name="memory.get_decisions")
async def memory_get_decisions(
    project: str,
    limit: int = 100,
) -> list[dict[str, Any]]:
    """Fetch decisions recorded for a project (workspace_path), newest first."""
    return await db.get_decisions(project=project, limit=max(1, min(limit, 500)))


@mcp.tool(name="memory.get_preferences")
async def memory_get_preferences(
    topic: str,
    limit: int = 10,
) -> list[dict[str, Any]]:
    """Semantic search over preferences, most relevant first."""
    embedding = await embeddings.embed(topic)
    return await db.get_preferences_by_embedding(
        embedding=embedding,
        limit=max(1, min(limit, 200)),
    )


if __name__ == "__main__":
    mcp.run()
