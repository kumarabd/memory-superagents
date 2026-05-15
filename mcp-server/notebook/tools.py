"""Register `notebook.*` tools for AgentLab workspace memory."""

from typing import Any

from fastmcp import FastMCP

from common import db


def register_notebook_tools(mcp: FastMCP) -> None:
    @mcp.tool(name="notebook.load")
    async def notebook_load(project: str) -> dict[str, Any]:
        """Load the full AgentLab notebook for a workspace (absolute project path).

        Returns JSON with keys: workspace_key, version, notebook (term_cache,
        concept_mapping, preferences, hypotheses, experiments, findings,
        semantic_links, open_questions, artifacts). If no row exists yet,
        version is 0 and notebook is the default empty shape.
        """
        return await db.notebook_load(project.strip())

    @mcp.tool(name="notebook.patch")
    async def notebook_patch(
        project: str,
        patch: dict[str, Any],
        expected_version: int | None = None,
    ) -> dict[str, Any]:
        """Merge top-level notebook keys into the stored payload for this workspace.

        Only these keys are applied when present in `patch`: term_cache,
        concept_mapping, preferences, hypotheses, experiments, findings,
        semantic_links, open_questions, artifacts. Each provided key replaces
        that subtree entirely (read-modify-write in the client).

        Optional `expected_version` enables optimistic locking (must match current
        row version, or 0 when no row exists yet).
        """
        return await db.notebook_patch(
            project.strip(), patch, expected_version
        )
