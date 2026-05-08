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
