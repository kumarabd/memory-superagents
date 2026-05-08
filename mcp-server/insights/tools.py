"""Register `insights.*` tools on a FastMCP instance."""

from typing import Any

from fastmcp import FastMCP

from common import db, embeddings


_PERSIST_SYNTHESIS_TOOL = "insights.persist_synthesis"


def register_insights_tools(mcp: FastMCP) -> None:
    @mcp.tool(name="insights.project_distribution")
    async def insights_project_distribution(
        project: str,
        include_inactive: bool = False,
    ) -> dict[str, Any]:
        """Active-memory counts by type for a workspace, plus totals and date span (viz-friendly JSON)."""
        return await db.project_memory_distribution(project, include_inactive=include_inactive)

    @mcp.tool(name="insights.persist_synthesis")
    async def insights_persist_synthesis(
        project: str,
        content: str,
        subject: str | None = None,
        source_memory_ids: list[str] | None = None,
        importance: float = 0.65,
        confidence: float = 0.75,
    ) -> dict[str, str]:
        """Store a derived pattern summary as `learning` with lineage pointing at source row ids."""
        embedding = await embeddings.embed(content)
        meta: dict[str, Any] = {
            "project": project,
            "workspace_path": project,
            "status": "active",
            "created_from": "derived",
            "source": "claude-code",
            "topic": "insights",
            "lineage": {
                "tool": _PERSIST_SYNTHESIS_TOOL,
                "source_memory_ids": list(source_memory_ids or []),
            },
        }
        memory_id = await db.write_memory(
            memory_type="learning",
            content=content,
            embedding=embedding,
            subject=subject,
            importance=importance,
            confidence=confidence,
            scope="project",
            metadata=meta,
        )
        return {"id": memory_id}
