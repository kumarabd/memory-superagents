# Memory CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `memory` CLI — the operational control plane for the memory platform — with commands for install, doctor, status, search, stats, export (including timeline), backup, restore, compact, reindex, config, and reset.

**Architecture:** A Typer CLI app in `cli/memory_cli/`, with shared utils for database access (async asyncpg wrapped with `asyncio.run`), Rich console output, and config from env vars. Each command is a separate module registered on the top-level `app`. The package is installed with `uv tool install --editable ./cli` and exposes the `memory` binary. The CLI connects directly to PostgreSQL — it does not go through the MCP server.

**Tech Stack:** Python 3.11+, Typer 0.12+, Rich 13+, asyncpg 0.29+, OpenAI 1.0+, Jinja2 3+, asyncio.

**Prerequisite:** Complete `2026-05-08-plugin-restructuring.md` first. Shared DB/embeddings modules live under `mcp-server/common/` (after the core/insights split; see `2026-05-08-core-insights-design.md`).

---

### Task 1: CLI package scaffold

**Files:**
- Create: `cli/pyproject.toml`
- Create: `cli/memory_cli/__init__.py`
- Create: `cli/memory_cli/main.py`
- Create: `cli/memory_cli/commands/__init__.py`
- Create: `cli/memory_cli/utils/__init__.py`

- [ ] **Step 1: Create cli/pyproject.toml**

```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "memory-cli"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = [
  "typer>=0.12",
  "rich>=13.0",
  "asyncpg>=0.29",
  "openai>=1.0",
  "jinja2>=3.0",
]

[project.scripts]
memory = "memory_cli.main:app"
```

- [ ] **Step 2: Create package init files**

`cli/memory_cli/__init__.py` — empty file:

```python
```

`cli/memory_cli/commands/__init__.py` — empty file:

```python
```

`cli/memory_cli/utils/__init__.py` — empty file:

```python
```

- [ ] **Step 3: Create cli/memory_cli/main.py**

```python
import typer
from memory_cli.commands import (
    install, uninstall, doctor, status,
    config, migrate, backup, restore,
    reindex, search, stats, compact, export, reset,
)

app = typer.Typer(
    name="memory",
    help="Claude Memory — operational control plane for your memory platform.",
    no_args_is_help=True,
)

app.add_typer(install.app,   name="install",   help="Install and configure the memory system.")
app.command("uninstall")(uninstall.run)
app.command("doctor")(doctor.run)
app.command("status")(status.run)
app.add_typer(config.app,    name="config",    help="Get or set configuration values.")
app.command("migrate")(migrate.run)
app.command("backup")(backup.run)
app.command("restore")(restore.run)
app.command("reindex")(reindex.run)
app.command("search")(search.run)
app.command("stats")(stats.run)
app.command("compact")(compact.run)
app.command("export")(export.run)
app.command("reset")(reset.run)

if __name__ == "__main__":
    app()
```

- [ ] **Step 4: Install the package in development mode**

```bash
uv tool install --editable ./cli
```

- [ ] **Step 5: Verify the binary exists**

```bash
memory --help
# Expected: Usage: memory [OPTIONS] COMMAND [ARGS]...
#           Commands: doctor, status, search, stats, ...
```

- [ ] **Step 6: Commit**

```bash
git add cli/
git commit -m "feat: scaffold memory-cli package with typer app and command registry"
```

---

### Task 2: Shared utils

**Files:**
- Create: `cli/memory_cli/utils/config.py`
- Create: `cli/memory_cli/utils/db.py`
- Create: `cli/memory_cli/utils/output.py`

- [ ] **Step 1: Create utils/config.py**

Reads credentials from environment. All CLI commands import `get_config()`.

```python
import os
from dataclasses import dataclass


@dataclass
class Config:
    database_url: str
    openai_api_key: str


def get_config() -> Config:
    db_url = os.environ.get("DATABASE_URL")
    if not db_url:
        from rich.console import Console
        Console().print("[red]✗[/red] DATABASE_URL is not set. Add it to your shell profile.")
        raise SystemExit(1)

    api_key = os.environ.get("OPENAI_API_KEY")
    if not api_key:
        from rich.console import Console
        Console().print("[red]✗[/red] OPENAI_API_KEY is not set. Add it to your shell profile.")
        raise SystemExit(1)

    return Config(database_url=db_url, openai_api_key=api_key)
```

- [ ] **Step 2: Create utils/db.py**

Async database helpers with a sync `run()` wrapper for use from Typer commands.

```python
import asyncio
import json
from typing import Any

import asyncpg


def run(coro):
    return asyncio.run(coro)


async def _setup_codecs(conn: asyncpg.Connection) -> None:
    await conn.set_type_codec("jsonb", encoder=json.dumps, decoder=json.loads, schema="pg_catalog")
    await conn.set_type_codec("json", encoder=json.dumps, decoder=json.loads, schema="pg_catalog")


async def connect(database_url: str) -> asyncpg.Connection:
    conn = await asyncpg.connect(database_url, timeout=10)
    await _setup_codecs(conn)
    return conn


def row(record: asyncpg.Record) -> dict[str, Any]:
    result = {}
    for key, value in record.items():
        if hasattr(value, "isoformat"):
            result[key] = value.isoformat()
        else:
            result[key] = value
    return result


async def fetch(database_url: str, query: str, *args) -> list[dict[str, Any]]:
    conn = await connect(database_url)
    try:
        rows = await conn.fetch(query, *args)
        return [row(r) for r in rows]
    finally:
        await conn.close()


async def fetchval(database_url: str, query: str, *args) -> Any:
    conn = await connect(database_url)
    try:
        return await conn.fetchval(query, *args)
    finally:
        await conn.close()


async def execute(database_url: str, query: str, *args) -> None:
    conn = await connect(database_url)
    try:
        await conn.execute(query, *args)
    finally:
        await conn.close()
```

- [ ] **Step 3: Create utils/output.py**

Shared Rich console and formatting helpers.

```python
from rich.console import Console
from rich.table import Table
from rich import box

console = Console()


def ok(msg: str) -> None:
    console.print(f"[green]✓[/green] {msg}")


def fail(msg: str) -> None:
    console.print(f"[red]✗[/red] {msg}")


def warn(msg: str) -> None:
    console.print(f"[yellow]![/yellow] {msg}")


def info(msg: str) -> None:
    console.print(f"[blue]·[/blue] {msg}")


def make_table(*columns: str, box_style=box.SIMPLE) -> Table:
    t = Table(box=box_style, show_header=True, header_style="bold cyan")
    for col in columns:
        t.add_column(col)
    return t
```

- [ ] **Step 4: Write tests for config and db utils**

Create `cli/tests/__init__.py` (empty) and `cli/tests/test_utils.py`:

```python
import os
import pytest
from unittest.mock import patch


def test_get_config_raises_when_db_url_missing():
    with patch.dict(os.environ, {}, clear=True):
        from memory_cli.utils.config import get_config
        with pytest.raises(SystemExit):
            get_config()


def test_get_config_raises_when_openai_key_missing():
    with patch.dict(os.environ, {"DATABASE_URL": "postgres://x"}, clear=True):
        from memory_cli.utils.config import get_config
        with pytest.raises(SystemExit):
            get_config()


def test_get_config_returns_values():
    env = {"DATABASE_URL": "postgres://x", "OPENAI_API_KEY": "sk-test"}
    with patch.dict(os.environ, env):
        from memory_cli.utils.config import get_config
        cfg = get_config()
        assert cfg.database_url == "postgres://x"
        assert cfg.openai_api_key == "sk-test"
```

Add pytest to cli/pyproject.toml:

```toml
[project.optional-dependencies]
dev = ["pytest>=8"]
```

- [ ] **Step 5: Run tests**

```bash
cd cli && uv run --extra dev pytest tests/ -v
# Expected: 3 passed
```

- [ ] **Step 6: Commit**

```bash
git add cli/memory_cli/utils/ cli/tests/
git commit -m "feat: add CLI utils — config, db helpers, rich output"
```

---

### Task 3: `memory doctor`

The most important command. Checks every component and prints a status table. Returns exit code 1 if any check fails.

**Files:**
- Create: `cli/memory_cli/commands/doctor.py`
- Create: `cli/tests/test_doctor.py`

- [ ] **Step 1: Write failing test**

```python
import pytest
from unittest.mock import AsyncMock, patch, MagicMock
from typer.testing import CliRunner
from memory_cli.main import app

runner = CliRunner()


def test_doctor_exits_1_on_db_failure():
    env = {"DATABASE_URL": "postgres://bad", "OPENAI_API_KEY": "sk-x"}
    with patch.dict("os.environ", env):
        result = runner.invoke(app, ["doctor"])
    assert result.exit_code == 1


def test_doctor_shows_check_labels():
    env = {"DATABASE_URL": "postgres://bad", "OPENAI_API_KEY": "sk-x"}
    with patch.dict("os.environ", env):
        result = runner.invoke(app, ["doctor"])
    assert "PostgreSQL" in result.output
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd cli && uv run --extra dev pytest tests/test_doctor.py -v
# Expected: FAIL — ImportError or AttributeError (doctor command not implemented)
```

- [ ] **Step 3: Implement commands/doctor.py**

```python
import asyncio
import subprocess
from pathlib import Path
from typing import Callable, Awaitable

import asyncpg
import typer
from rich.table import Table
from rich import box

from memory_cli.utils.config import get_config
from memory_cli.utils.output import console


async def _check_postgres(db_url: str) -> tuple[bool, str]:
    try:
        conn = await asyncpg.connect(db_url, timeout=5)
        await conn.close()
        return True, "reachable"
    except Exception as e:
        return False, f"unreachable — {e}"


async def _check_pgvector(db_url: str) -> tuple[bool, str]:
    try:
        conn = await asyncpg.connect(db_url, timeout=5)
        version = await conn.fetchval(
            "SELECT extversion FROM pg_extension WHERE extname = 'vector'"
        )
        await conn.close()
        if version:
            return True, f"v{version} installed"
        return False, "extension missing — run: CREATE EXTENSION vector;"
    except Exception as e:
        return False, str(e)


async def _check_schema(db_url: str) -> tuple[bool, str]:
    try:
        conn = await asyncpg.connect(db_url, timeout=5)
        tables = {
            r["tablename"]
            for r in await conn.fetch(
                "SELECT tablename FROM pg_tables WHERE schemaname = 'public'"
            )
        }
        await conn.close()
        required = {"memory_events", "conversation_sessions", "messages"}
        missing = required - tables
        if missing:
            return False, f"missing tables: {', '.join(sorted(missing))}"
        return True, "all tables present"
    except Exception as e:
        return False, str(e)


def _check_mcp() -> tuple[bool, str]:
    try:
        result = subprocess.run(
            ["claude", "mcp", "list"],
            capture_output=True, text=True, timeout=10
        )
        lines = result.stdout
        if "memory" in lines and "✗" not in lines.split("memory")[1].split("\n")[0]:
            return True, "registered and connected"
        if "memory" in lines:
            return False, "registered but not connected — check env vars"
        return False, "not registered — run: ./install.sh"
    except FileNotFoundError:
        return False, "claude CLI not found"
    except Exception as e:
        return False, str(e)


async def _check_embeddings(api_key: str) -> tuple[bool, str]:
    try:
        from openai import AsyncOpenAI
        client = AsyncOpenAI(api_key=api_key, timeout=10.0)
        resp = await client.embeddings.create(
            model="text-embedding-ada-002", input="doctor check"
        )
        await client.close()
        dim = len(resp.data[0].embedding)
        return True, f"text-embedding-ada-002 ({dim}d)"
    except Exception as e:
        return False, str(e)


async def _check_roundtrip(db_url: str, api_key: str) -> tuple[bool, str]:
    try:
        import json
        from openai import AsyncOpenAI

        client = AsyncOpenAI(api_key=api_key, timeout=10.0)
        resp = await client.embeddings.create(
            model="text-embedding-ada-002", input="roundtrip check"
        )
        emb = resp.data[0].embedding
        await client.close()

        conn = await asyncpg.connect(db_url, timeout=5)
        await conn.set_type_codec(
            "jsonb", encoder=json.dumps, decoder=json.loads, schema="pg_catalog"
        )
        vec = "[" + ",".join(str(x) for x in emb) + "]"
        row = await conn.fetchrow(
            """INSERT INTO memory_events
                   (memory_type, content, scope, embedding, metadata)
               VALUES ('observation', 'doctor:roundtrip', 'temporary', $1::vector,
                       '{"status":"active","source":"doctor"}'::jsonb)
               RETURNING id""",
            vec,
        )
        await conn.execute(
            "DELETE FROM memory_events WHERE id = $1", row["id"]
        )
        await conn.close()
        return True, "write → read → delete OK"
    except Exception as e:
        return False, str(e)


def run() -> None:
    cfg = get_config()

    console.print("\n[bold]Claude Memory — Doctor[/bold]\n")

    # Collect results
    async def gather():
        results = []
        for label, coro in [
            ("PostgreSQL reachable",    _check_postgres(cfg.database_url)),
            ("pgvector extension",      _check_pgvector(cfg.database_url)),
            ("Schema tables",           _check_schema(cfg.database_url)),
            ("MCP server",              asyncio.coroutine(lambda: _check_mcp())()),
            ("Embeddings API",          _check_embeddings(cfg.openai_api_key)),
            ("Write/read roundtrip",    _check_roundtrip(cfg.database_url, cfg.openai_api_key)),
        ]:
            ok, detail = await coro
            results.append((label, ok, detail))
        return results

    # _check_mcp is sync; wrap it
    async def gather_all():
        results = []
        checks_async = [
            ("PostgreSQL reachable",  _check_postgres(cfg.database_url)),
            ("pgvector extension",    _check_pgvector(cfg.database_url)),
            ("Schema tables",         _check_schema(cfg.database_url)),
            ("Embeddings API",        _check_embeddings(cfg.openai_api_key)),
            ("Write/read roundtrip",  _check_roundtrip(cfg.database_url, cfg.openai_api_key)),
        ]
        async_results = await asyncio.gather(
            *[coro for _, coro in checks_async], return_exceptions=True
        )
        for (label, _), result in zip(checks_async, async_results):
            if isinstance(result, Exception):
                results.append((label, False, str(result)))
            else:
                results.append((label, *result))

        # Sync check
        ok, detail = _check_mcp()
        results.insert(3, ("MCP server", ok, detail))
        return results

    results = asyncio.run(gather_all())

    table = Table(box=box.SIMPLE, show_header=True, header_style="bold")
    table.add_column("Check", style="bold")
    table.add_column("Status", justify="center")
    table.add_column("Detail")

    all_ok = True
    for label, ok, detail in results:
        status = "[green]✓[/green]" if ok else "[red]✗[/red]"
        table.add_row(label, status, detail)
        if not ok:
            all_ok = False

    console.print(table)

    if all_ok:
        console.print("[green]All checks passed.[/green]\n")
    else:
        console.print("[red]Some checks failed. Fix the issues above and re-run memory doctor.[/red]\n")
        raise typer.Exit(1)
```

- [ ] **Step 4: Run tests**

```bash
cd cli && uv run --extra dev pytest tests/test_doctor.py -v
# Expected: 2 passed
```

- [ ] **Step 5: Smoke test against live system**

```bash
memory doctor
# Expected: table with ✓ for all rows, "All checks passed."
```

- [ ] **Step 6: Commit**

```bash
git add cli/memory_cli/commands/doctor.py cli/tests/test_doctor.py
git commit -m "feat: implement memory doctor — full system health check"
```

---

### Task 4: `memory status`

Fast operational snapshot — DB size, memory count, active projects, last write.

**Files:**
- Create: `cli/memory_cli/commands/status.py`

- [ ] **Step 1: Create commands/status.py**

```python
import asyncio
from memory_cli.utils.config import get_config
from memory_cli.utils.db import fetch, fetchval
from memory_cli.utils.output import console, make_table


def run() -> None:
    cfg = get_config()

    async def gather():
        total, db_size, last_write, project_count = await asyncio.gather(
            fetchval(cfg.database_url,
                "SELECT count(*) FROM memory_events WHERE metadata->>'status' = 'active'"),
            fetchval(cfg.database_url,
                "SELECT pg_size_pretty(pg_database_size(current_database()))"),
            fetchval(cfg.database_url,
                "SELECT max(created_at) FROM memory_events"),
            fetchval(cfg.database_url,
                """SELECT count(DISTINCT coalesce(metadata->>'project', metadata->>'workspace_path'))
                   FROM memory_events
                   WHERE metadata->>'project' IS NOT NULL
                     AND metadata->>'status' = 'active'"""),
        )
        return total, db_size, last_write, project_count

    total, db_size, last_write, project_count = asyncio.run(gather())

    console.print("\n[bold]Claude Memory — Status[/bold]\n")
    t = make_table("Metric", "Value")
    t.add_row("Active memories", str(total or 0))
    t.add_row("Database size", str(db_size or "—"))
    t.add_row("Active projects", str(project_count or 0))
    t.add_row("Last write", str(last_write)[:19] if last_write else "never")
    console.print(t)
    console.print()
```

- [ ] **Step 2: Smoke test**

```bash
memory status
# Expected: table with Active memories, Database size, Active projects, Last write
```

- [ ] **Step 3: Commit**

```bash
git add cli/memory_cli/commands/status.py
git commit -m "feat: implement memory status — operational snapshot"
```

---

### Task 5: `memory stats`

Full breakdown by type, scope, topic, and project. The analytics dashboard.

**Files:**
- Create: `cli/memory_cli/commands/stats.py`

- [ ] **Step 1: Create commands/stats.py**

```python
import asyncio
from memory_cli.utils.config import get_config
from memory_cli.utils.db import fetch, fetchval
from memory_cli.utils.output import console, make_table
from rich.columns import Columns
from rich.panel import Panel


def run() -> None:
    cfg = get_config()

    async def gather():
        by_type, by_scope, by_topic, by_project, by_day = await asyncio.gather(
            fetch(cfg.database_url,
                """SELECT memory_type, count(*) AS n
                   FROM memory_events
                   WHERE metadata->>'status' = 'active'
                   GROUP BY memory_type ORDER BY n DESC"""),
            fetch(cfg.database_url,
                """SELECT scope, count(*) AS n
                   FROM memory_events
                   WHERE metadata->>'status' = 'active'
                   GROUP BY scope ORDER BY n DESC"""),
            fetch(cfg.database_url,
                """SELECT metadata->>'topic' AS topic, count(*) AS n
                   FROM memory_events
                   WHERE metadata->>'topic' IS NOT NULL
                     AND metadata->>'status' = 'active'
                   GROUP BY topic ORDER BY n DESC LIMIT 10"""),
            fetch(cfg.database_url,
                """SELECT coalesce(metadata->>'project', metadata->>'workspace_path') AS project,
                          count(*) AS n
                   FROM memory_events
                   WHERE coalesce(metadata->>'project', metadata->>'workspace_path') IS NOT NULL
                     AND metadata->>'status' = 'active'
                   GROUP BY project ORDER BY n DESC LIMIT 10"""),
            fetch(cfg.database_url,
                """SELECT date_trunc('day', created_at)::date AS day, count(*) AS n
                   FROM memory_events
                   WHERE created_at > now() - interval '14 days'
                   GROUP BY day ORDER BY day DESC"""),
        )
        return by_type, by_scope, by_topic, by_project, by_day

    by_type, by_scope, by_topic, by_project, by_day = asyncio.run(gather())

    console.print("\n[bold]Claude Memory — Statistics[/bold]\n")

    t_type = make_table("Type", "Count")
    for r in by_type:
        t_type.add_row(r["memory_type"], str(r["n"]))

    t_scope = make_table("Scope", "Count")
    for r in by_scope:
        t_scope.add_row(r["scope"], str(r["n"]))

    t_topic = make_table("Topic", "Count")
    for r in by_topic:
        t_topic.add_row(r["topic"] or "—", str(r["n"]))

    t_project = make_table("Project", "Count")
    for r in by_project:
        name = (r["project"] or "—").split("/")[-1]  # show basename
        t_project.add_row(name, str(r["n"]))

    t_activity = make_table("Date", "Writes")
    for r in by_day:
        t_activity.add_row(str(r["day"]), str(r["n"]))

    console.print(Columns([
        Panel(t_type,     title="By type",    expand=False),
        Panel(t_scope,    title="By scope",   expand=False),
        Panel(t_topic,    title="Top topics", expand=False),
    ]))
    console.print(Columns([
        Panel(t_project,  title="Top projects",     expand=False),
        Panel(t_activity, title="Activity (14d)",   expand=False),
    ]))
    console.print()
```

- [ ] **Step 2: Smoke test**

```bash
memory stats
# Expected: panels with by-type, by-scope, top topics, top projects, activity
```

- [ ] **Step 3: Commit**

```bash
git add cli/memory_cli/commands/stats.py
git commit -m "feat: implement memory stats — analytics dashboard"
```

---

### Task 6: `memory search`

Semantic search with optional type and project filters. Useful for debugging — lets users see what Claude is actually remembering.

**Files:**
- Create: `cli/memory_cli/commands/search.py`

- [ ] **Step 1: Create commands/search.py**

```python
import asyncio
import json
from typing import Optional

import asyncpg
import typer
from openai import AsyncOpenAI

from memory_cli.utils.config import get_config
from memory_cli.utils.output import console, make_table


def run(
    query: str = typer.Argument(..., help="Natural language search query"),
    type: Optional[str] = typer.Option(None, "--type", "-t", help="Filter by memory_type"),
    project: Optional[str] = typer.Option(None, "--project", "-p", help="Filter by project path"),
    limit: int = typer.Option(10, "--limit", "-n", help="Max results"),
) -> None:
    cfg = get_config()

    async def _search():
        client = AsyncOpenAI(api_key=cfg.openai_api_key, timeout=15.0)
        resp = await client.embeddings.create(model="text-embedding-ada-002", input=query)
        emb = resp.data[0].embedding
        await client.close()

        vec = "[" + ",".join(str(x) for x in emb) + "]"

        conn = await asyncpg.connect(cfg.database_url, timeout=10)
        await conn.set_type_codec(
            "jsonb", encoder=json.dumps, decoder=json.loads, schema="pg_catalog"
        )
        rows = await conn.fetch(
            f"""
            SELECT
                m.memory_type,
                m.subject,
                m.content,
                m.importance,
                m.scope,
                m.created_at,
                1 - (m.embedding <=> $1::vector) AS similarity
            FROM memory_events m
            WHERE ($2::text IS NULL OR m.memory_type = $2)
              AND (
                $3::text IS NULL
                OR m.metadata->>'project' = $3
                OR m.metadata->>'workspace_path' = $3
              )
              AND (m.metadata->>'status' IS NULL OR m.metadata->>'status' = 'active')
            ORDER BY m.embedding <=> $1::vector
            LIMIT $4
            """,
            vec, type, project, max(1, min(limit, 100)),
        )
        await conn.close()
        return rows

    rows = asyncio.run(_search())

    if not rows:
        console.print("[yellow]No results found.[/yellow]")
        return

    console.print(f"\n[bold]Results for:[/bold] {query}\n")
    t = make_table("Score", "Type", "Subject", "Content", "Scope", "Date")
    for r in rows:
        score = f"{r['similarity']:.2f}"
        date = str(r["created_at"])[:10]
        content = (r["content"] or "")[:80] + ("…" if len(r["content"] or "") > 80 else "")
        t.add_row(score, r["memory_type"], r["subject"] or "—", content, r["scope"], date)
    console.print(t)
    console.print()
```

- [ ] **Step 2: Smoke test**

```bash
memory search "postgres database decisions"
# Expected: table with similarity scores, types, subjects, truncated content
memory search "kubernetes" --type decision
# Expected: filtered to decision type
```

- [ ] **Step 3: Commit**

```bash
git add cli/memory_cli/commands/search.py
git commit -m "feat: implement memory search — semantic search with type/project filters"
```

---

### Task 7: `memory export`

Export memories in three formats: `json` (raw), `markdown` (readable), and `timeline` (month-by-month narrative from decisions and summaries).

**Files:**
- Create: `cli/memory_cli/commands/export.py`
- Create: `cli/memory_cli/templates/timeline.md.jinja2`
- Create: `cli/memory_cli/templates/export.md.jinja2`

- [ ] **Step 1: Create timeline template**

`cli/memory_cli/templates/timeline.md.jinja2`:

```jinja2
# Memory Timeline

_Generated {{ generated_at }}_

---
{% for month, entries in months.items() %}
## {{ month }}
{% for entry in entries %}
- {{ entry.content | truncate(120, True, '…') }}
{% endfor %}
{% endfor %}
```

- [ ] **Step 2: Create markdown export template**

`cli/memory_cli/templates/export.md.jinja2`:

```jinja2
# Memory Export

_Generated {{ generated_at }} — {{ total }} memories_

---
{% for type, memories in by_type.items() %}
## {{ type | title | replace('_', ' ') }} ({{ memories | length }})
{% for m in memories %}
### {{ m.subject or '(no subject)' }}
{{ m.content }}

_scope: {{ m.scope }} | importance: {{ m.importance }} | {{ m.created_at[:10] }}_

---
{% endfor %}
{% endfor %}
```

- [ ] **Step 3: Create commands/export.py**

```python
import asyncio
import json
from datetime import datetime
from pathlib import Path
from typing import Optional

import typer
from jinja2 import Environment, PackageLoader

from memory_cli.utils.config import get_config
from memory_cli.utils.db import fetch
from memory_cli.utils.output import console, ok


_TEMPLATES = Environment(
    loader=PackageLoader("memory_cli", "templates"), autoescape=False
)


def run(
    format: str = typer.Option("json", "--format", "-f",
        help="Output format: json | markdown | timeline"),
    project: Optional[str] = typer.Option(None, "--project", "-p",
        help="Filter to a specific project path"),
    output: Optional[Path] = typer.Option(None, "--output", "-o",
        help="Write to file instead of stdout"),
) -> None:
    cfg = get_config()

    if format not in ("json", "markdown", "timeline"):
        console.print("[red]--format must be json, markdown, or timeline[/red]")
        raise typer.Exit(1)

    async def _fetch():
        project_filter = (
            "AND (m.metadata->>'project' = $1 OR m.metadata->>'workspace_path' = $1)"
            if project else ""
        )
        args = [project] if project else []
        return await fetch(
            cfg.database_url,
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
                m.metadata
            FROM memory_events m
            WHERE (m.metadata->>'status' IS NULL OR m.metadata->>'status' = 'active')
              {project_filter}
            ORDER BY m.created_at ASC
            """,
            *args,
        )

    memories = asyncio.run(_fetch())
    now = datetime.utcnow().strftime("%Y-%m-%d %H:%M UTC")

    if format == "json":
        content = json.dumps(memories, indent=2, default=str)

    elif format == "markdown":
        by_type: dict = {}
        for m in memories:
            by_type.setdefault(m["memory_type"], []).append(m)
        tmpl = _TEMPLATES.get_template("export.md.jinja2")
        content = tmpl.render(
            generated_at=now,
            total=len(memories),
            by_type=by_type,
        )

    else:  # timeline
        months: dict = {}
        relevant_types = {"decision", "conversation_summary", "learning", "project_context"}
        for m in memories:
            if m["memory_type"] not in relevant_types:
                continue
            month = str(m["created_at"])[:7]  # "2026-05"
            months.setdefault(month, []).append(m)
        tmpl = _TEMPLATES.get_template("timeline.md.jinja2")
        content = tmpl.render(generated_at=now, months=months)

    if output:
        output.write_text(content)
        ok(f"Exported {len(memories)} memories to {output}")
    else:
        console.print(content)
```

- [ ] **Step 4: Smoke test all three formats**

```bash
memory export --format json    | head -30
# Expected: JSON array of memory objects

memory export --format markdown | head -30
# Expected: Markdown with ## headers per type

memory export --format timeline
# Expected: ## 2026-05 section with decision/summary bullets
```

- [ ] **Step 5: Commit**

```bash
git add cli/memory_cli/commands/export.py cli/memory_cli/templates/
git commit -m "feat: implement memory export — json, markdown, and timeline formats"
```

---

### Task 8: `memory backup` and `memory restore`

Uses `pg_dump` / `pg_restore` via `docker exec` to dump and restore the entire database.

**Files:**
- Create: `cli/memory_cli/commands/backup.py`
- Create: `cli/memory_cli/commands/restore.py`

- [ ] **Step 1: Create commands/backup.py**

```python
import subprocess
import sys
from datetime import datetime
from pathlib import Path
from typing import Optional

import typer

from memory_cli.utils.output import console, ok, fail


def run(
    output: Optional[Path] = typer.Option(None, "--output", "-o",
        help="Backup file path (default: memory-backup-YYYY-MM-DD.sql)"),
) -> None:
    if output is None:
        output = Path(f"memory-backup-{datetime.now().strftime('%Y-%m-%d-%H%M%S')}.sql")

    console.print(f"[bold]Backing up to {output}...[/bold]")

    result = subprocess.run(
        ["docker", "exec", "claude-memory-db",
         "pg_dump", "-U", "postgres", "-d", "claude_memory", "--no-owner"],
        capture_output=True,
    )

    if result.returncode != 0:
        fail(f"pg_dump failed: {result.stderr.decode()}")
        raise typer.Exit(1)

    output.write_bytes(result.stdout)
    size = output.stat().st_size
    ok(f"Backup written to {output} ({size:,} bytes)")
```

- [ ] **Step 2: Create commands/restore.py**

```python
import subprocess
from pathlib import Path

import typer

from memory_cli.utils.output import console, ok, fail, warn


def run(
    file: Path = typer.Argument(..., help="Backup file to restore from"),
    yes: bool = typer.Option(False, "--yes", "-y", help="Skip confirmation"),
) -> None:
    if not file.exists():
        fail(f"File not found: {file}")
        raise typer.Exit(1)

    if not yes:
        warn("This will DROP and recreate the claude_memory database.")
        typer.confirm("Continue?", abort=True)

    console.print(f"[bold]Restoring from {file}...[/bold]")

    # Drop and recreate the database
    for cmd in [
        ["docker", "exec", "claude-memory-db",
         "psql", "-U", "postgres", "-c",
         "DROP DATABASE IF EXISTS claude_memory;"],
        ["docker", "exec", "claude-memory-db",
         "psql", "-U", "postgres", "-c",
         "CREATE DATABASE claude_memory;"],
    ]:
        result = subprocess.run(cmd, capture_output=True)
        if result.returncode != 0:
            fail(result.stderr.decode())
            raise typer.Exit(1)

    result = subprocess.run(
        ["docker", "exec", "-i", "claude-memory-db",
         "psql", "-U", "postgres", "-d", "claude_memory"],
        input=file.read_bytes(),
        capture_output=True,
    )

    if result.returncode != 0:
        fail(f"Restore failed: {result.stderr.decode()}")
        raise typer.Exit(1)

    ok(f"Restored from {file}")
```

- [ ] **Step 3: Smoke test backup**

```bash
memory backup
# Expected: memory-backup-2026-05-08-HHMMSS.sql created with size output
ls memory-backup-*.sql
```

- [ ] **Step 4: Commit**

```bash
git add cli/memory_cli/commands/backup.py cli/memory_cli/commands/restore.py
git commit -m "feat: implement memory backup and restore via pg_dump/pg_restore"
```

---

### Task 9: `memory compact`, `memory reindex`, `memory migrate`

**Files:**
- Create: `cli/memory_cli/commands/compact.py`
- Create: `cli/memory_cli/commands/reindex.py`
- Create: `cli/memory_cli/commands/migrate.py`

- [ ] **Step 1: Create commands/compact.py**

Marks memories older than `--days` with importance below `--threshold` as `stale`. Also marks all `superseded` memories older than `--days` as `stale` to remove them from searches.

```python
import asyncio
from typing import Optional

import typer

from memory_cli.utils.config import get_config
from memory_cli.utils.db import fetchval, execute
from memory_cli.utils.output import console, ok


def run(
    days: int = typer.Option(90, "--days", "-d",
        help="Archive memories older than this many days"),
    threshold: float = typer.Option(0.4, "--threshold",
        help="Archive active memories with importance below this value"),
    dry_run: bool = typer.Option(False, "--dry-run",
        help="Show what would be archived without changing anything"),
) -> None:
    cfg = get_config()

    async def _compact():
        count_stale = await fetchval(
            cfg.database_url,
            """SELECT count(*) FROM memory_events
               WHERE (metadata->>'status' = 'superseded' OR importance < $1)
                 AND created_at < now() - ($2 || ' days')::interval""",
            threshold, str(days),
        )

        if dry_run:
            console.print(f"[yellow]Dry run:[/yellow] would archive {count_stale} memories.")
            return count_stale

        await execute(
            cfg.database_url,
            """UPDATE memory_events
               SET metadata = jsonb_set(metadata, '{status}', '"stale"')
               WHERE (metadata->>'status' = 'superseded' OR importance < $1)
                 AND created_at < now() - ($2 || ' days')::interval
                 AND (metadata->>'status' IS NULL OR metadata->>'status' != 'stale')""",
            threshold, str(days),
        )
        return count_stale

    count = asyncio.run(_compact())
    if not dry_run:
        ok(f"Archived {count} memories (marked as stale).")
```

- [ ] **Step 2: Create commands/reindex.py**

Re-embeds all memories. Runs in batches of 100 to avoid rate limits.

```python
import asyncio
import json

import asyncpg
import typer
from openai import AsyncOpenAI
from rich.progress import Progress, SpinnerColumn, BarColumn, TextColumn

from memory_cli.utils.config import get_config
from memory_cli.utils.output import console, ok, info


def run(
    batch_size: int = typer.Option(50, "--batch-size", help="Embeddings per API call"),
) -> None:
    cfg = get_config()

    async def _reindex():
        conn = await asyncpg.connect(cfg.database_url, timeout=10)
        await conn.set_type_codec(
            "jsonb", encoder=json.dumps, decoder=json.loads, schema="pg_catalog"
        )
        rows = await conn.fetch(
            "SELECT id, content FROM memory_events WHERE content IS NOT NULL ORDER BY created_at"
        )
        total = len(rows)
        info(f"Re-embedding {total} memories in batches of {batch_size}...")

        client = AsyncOpenAI(api_key=cfg.openai_api_key, timeout=30.0)

        with Progress(
            SpinnerColumn(), BarColumn(), TextColumn("{task.completed}/{task.total}"),
            transient=True,
        ) as progress:
            task = progress.add_task("Re-embedding", total=total)

            for i in range(0, total, batch_size):
                batch = rows[i : i + batch_size]
                texts = [r["content"] for r in batch]
                resp = await client.embeddings.create(
                    model="text-embedding-ada-002", input=texts
                )
                for row, emb_data in zip(batch, resp.data):
                    vec = "[" + ",".join(str(x) for x in emb_data.embedding) + "]"
                    await conn.execute(
                        "UPDATE memory_events SET embedding = $1::vector WHERE id = $2",
                        vec, row["id"],
                    )
                progress.advance(task, len(batch))

        await client.close()
        await conn.close()
        return total

    total = asyncio.run(_reindex())
    ok(f"Re-indexed {total} memories.")
```

- [ ] **Step 3: Create commands/migrate.py**

Applies any `.sql` files in `migrations/` that haven't been applied yet, tracked in a `schema_migrations` table.

```python
import asyncio
import hashlib
import json
from pathlib import Path

import asyncpg

from memory_cli.utils.config import get_config
from memory_cli.utils.output import console, ok, info


_MIGRATIONS_DIR = Path(__file__).parent.parent.parent.parent.parent / "migrations"


def run() -> None:
    cfg = get_config()

    async def _migrate():
        conn = await asyncpg.connect(cfg.database_url, timeout=10)
        await conn.set_type_codec(
            "jsonb", encoder=json.dumps, decoder=json.loads, schema="pg_catalog"
        )

        # Create migrations tracking table if it doesn't exist
        await conn.execute("""
            CREATE TABLE IF NOT EXISTS schema_migrations (
                filename text PRIMARY KEY,
                applied_at timestamptz DEFAULT now()
            )
        """)

        applied = {
            r["filename"]
            for r in await conn.fetch("SELECT filename FROM schema_migrations")
        }

        migration_files = sorted(_MIGRATIONS_DIR.glob("*.sql"))
        pending = [f for f in migration_files if f.name not in applied]

        if not pending:
            info("All migrations already applied.")
            await conn.close()
            return 0

        for f in pending:
            console.print(f"  Applying [bold]{f.name}[/bold]...")
            sql = f.read_text()
            await conn.execute(sql)
            await conn.execute(
                "INSERT INTO schema_migrations (filename) VALUES ($1)", f.name
            )
            ok(f"Applied {f.name}")

        await conn.close()
        return len(pending)

    count = asyncio.run(_migrate())
    if count:
        ok(f"{count} migration(s) applied.")
```

- [ ] **Step 4: Smoke test**

```bash
memory migrate
# Expected: "All migrations already applied." (schema already applied)

memory compact --dry-run
# Expected: "Dry run: would archive N memories."

memory compact
# Expected: "Archived N memories."
```

- [ ] **Step 5: Commit**

```bash
git add cli/memory_cli/commands/compact.py \
        cli/memory_cli/commands/reindex.py \
        cli/memory_cli/commands/migrate.py
git commit -m "feat: implement memory compact, reindex, and migrate"
```

---

### Task 10: `memory config`, `memory reset`, `memory uninstall`

**Files:**
- Create: `cli/memory_cli/commands/config.py`
- Create: `cli/memory_cli/commands/reset.py`
- Create: `cli/memory_cli/commands/uninstall.py`
- Create: `cli/memory_cli/commands/install.py`

- [ ] **Step 1: Create commands/config.py**

Shows current config (env vars). Typer sub-app for future `config set` support.

```python
import os
import typer
from memory_cli.utils.output import console, make_table

app = typer.Typer(help="View configuration.")


@app.callback(invoke_without_command=True)
def show(ctx: typer.Context) -> None:
    if ctx.invoked_subcommand:
        return
    t = make_table("Variable", "Value", "Status")
    db_url = os.environ.get("DATABASE_URL", "")
    api_key = os.environ.get("OPENAI_API_KEY", "")
    t.add_row("DATABASE_URL",   db_url   or "[red]not set[/red]", "✓" if db_url   else "✗")
    t.add_row("OPENAI_API_KEY", "sk-…" + api_key[-4:] if api_key else "[red]not set[/red]",
              "✓" if api_key else "✗")
    console.print(t)
```

- [ ] **Step 2: Create commands/reset.py**

Nuclear option: truncates all memory_events rows.

```python
import asyncio
import typer
from memory_cli.utils.config import get_config
from memory_cli.utils.db import execute, fetchval
from memory_cli.utils.output import console, ok, warn


def run(
    yes: bool = typer.Option(False, "--yes", "-y", help="Skip confirmation"),
    project: str = typer.Option(None, "--project", "-p",
        help="Delete only memories for this project path"),
) -> None:
    cfg = get_config()

    async def _count():
        if project:
            return await fetchval(
                cfg.database_url,
                "SELECT count(*) FROM memory_events WHERE metadata->>'project' = $1",
                project,
            )
        return await fetchval(cfg.database_url, "SELECT count(*) FROM memory_events")

    count = asyncio.run(_count())
    scope = f"project {project}" if project else "ALL workspaces"
    warn(f"This will permanently delete {count} memories from {scope}.")

    if not yes:
        typer.confirm("Continue?", abort=True)

    async def _delete():
        if project:
            await execute(
                cfg.database_url,
                "DELETE FROM memory_events WHERE metadata->>'project' = $1",
                project,
            )
        else:
            await execute(cfg.database_url, "TRUNCATE memory_events")

    asyncio.run(_delete())
    ok(f"Deleted {count} memories from {scope}.")
```

- [ ] **Step 3: Create commands/uninstall.py**

```python
import subprocess
import typer
from memory_cli.utils.output import console, ok, warn


def run(
    keep_data: bool = typer.Option(False, "--keep-data",
        help="Keep the PostgreSQL database (just unregister the MCP server)"),
    yes: bool = typer.Option(False, "--yes", "-y"),
) -> None:
    if not yes:
        warn("This will unregister the MCP server from Claude Code.")
        if not keep_data:
            warn("The PostgreSQL container and all data will also be removed.")
        typer.confirm("Continue?", abort=True)

    result = subprocess.run(
        ["claude", "mcp", "remove", "memory", "-s", "user"],
        capture_output=True, text=True,
    )
    if result.returncode == 0:
        ok("MCP server unregistered from Claude Code.")
    else:
        console.print(f"[yellow]Could not unregister MCP: {result.stderr}[/yellow]")

    if not keep_data:
        subprocess.run(["docker", "compose", "down", "-v"], check=False)
        ok("PostgreSQL container and volume removed.")

    console.print("\nTo reinstall: [bold]./install.sh[/bold]")
```

- [ ] **Step 4: Create commands/install.py (CLI wrapper for install.sh)**

```python
import subprocess
from pathlib import Path
import typer

from memory_cli.utils.output import console, info

app = typer.Typer(help="Install the memory system.")


@app.callback(invoke_without_command=True)
def run(ctx: typer.Context) -> None:
    if ctx.invoked_subcommand:
        return
    install_sh = Path(__file__).parent.parent.parent.parent.parent / "install.sh"
    if not install_sh.exists():
        console.print(f"[red]install.sh not found at {install_sh}[/red]")
        raise typer.Exit(1)
    info(f"Running {install_sh}...")
    subprocess.run(["bash", str(install_sh)], check=True)
```

- [ ] **Step 5: Smoke test**

```bash
memory config
# Expected: table with DATABASE_URL and OPENAI_API_KEY values

memory reset --project /nonexistent --yes
# Expected: "Deleted 0 memories from project /nonexistent."
```

- [ ] **Step 6: Commit**

```bash
git add cli/memory_cli/commands/config.py \
        cli/memory_cli/commands/reset.py \
        cli/memory_cli/commands/uninstall.py \
        cli/memory_cli/commands/install.py
git commit -m "feat: implement memory config, reset, uninstall, install"
```

---

### Task 11: Wire all commands into main.py and integration test

**Files:**
- Modify: `cli/memory_cli/main.py`

- [ ] **Step 1: Verify main.py has all commands registered**

Full content of `cli/memory_cli/main.py`:

```python
import typer

from memory_cli.commands import (
    install, uninstall, doctor, status,
    config, migrate, backup, restore,
    reindex, search, stats, compact, export, reset,
)

app = typer.Typer(
    name="memory",
    help="Claude Memory — operational control plane for your memory platform.",
    no_args_is_help=True,
)

app.add_typer(install.app,  name="install",   help="Install the memory system.")
app.add_typer(config.app,   name="config",    help="Show configuration.")
app.command("uninstall")(uninstall.run)
app.command("doctor")(doctor.run)
app.command("status")(status.run)
app.command("migrate")(migrate.run)
app.command("backup")(backup.run)
app.command("restore")(restore.run)
app.command("reindex")(reindex.run)
app.command("search")(search.run)
app.command("stats")(stats.run)
app.command("compact")(compact.run)
app.command("export")(export.run)
app.command("reset")(reset.run)

if __name__ == "__main__":
    app()
```

- [ ] **Step 2: Run full command listing**

```bash
memory --help
# Expected: lists all 14 commands with descriptions
```

- [ ] **Step 3: Run the golden path**

```bash
memory doctor && memory status && memory stats && memory search "test" -n 3
# Expected: all four commands produce output without errors
```

- [ ] **Step 4: Reinstall CLI to pick up all changes**

```bash
uv tool install --editable ./cli --reinstall
memory --help
# Expected: all commands listed
```

- [ ] **Step 5: Commit**

```bash
git add cli/memory_cli/main.py
git commit -m "feat: wire all 14 commands into memory CLI — complete control plane"
```

---

### Final CLI structure

```
cli/
├── pyproject.toml
└── memory_cli/
    ├── __init__.py
    ├── main.py
    ├── commands/
    │   ├── __init__.py
    │   ├── install.py
    │   ├── uninstall.py
    │   ├── doctor.py
    │   ├── status.py
    │   ├── config.py
    │   ├── migrate.py
    │   ├── backup.py
    │   ├── restore.py
    │   ├── reindex.py
    │   ├── search.py
    │   ├── stats.py
    │   ├── compact.py
    │   ├── export.py
    │   └── reset.py
    ├── utils/
    │   ├── __init__.py
    │   ├── config.py
    │   ├── db.py
    │   └── output.py
    └── templates/
        ├── export.md.jinja2
        └── timeline.md.jinja2
```
