import os
from contextlib import asynccontextmanager

from fastmcp import FastMCP

from common import db, embeddings
from core.tools import register_core_tools
from insights.tools import register_insights_tools
from notebook.tools import register_notebook_tools


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
    await db.create_session(
        session_id=os.environ.get("CLAUDE_CODE_SESSION_ID"),
        workspace_path=os.getcwd(),
    )
    yield
    await db.close_session()
    await db.close_pool()
    await embeddings.close()


mcp = FastMCP("memory", lifespan=lifespan)

register_core_tools(mcp)
register_insights_tools(mcp)
register_notebook_tools(mcp)

if __name__ == "__main__":
    mcp.run()
