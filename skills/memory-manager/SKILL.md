---
name: memory-manager
description: Use this skill whenever the user asks about prior work, preferences, projects, architecture decisions, debugging history, or when beginning/ending substantial work.
---

# Memory Manager

Use the `memory.*` MCP tools to retrieve context, apply learned knowledge, and persist new memory across sessions. Memory is divided into two tracks that must both be loaded: **global** (who you are, how you work) and **project** (what's happening here).

---

## Memory Anatomy

Every memory event has:

| Field | Purpose |
|---|---|
| `memory_type` | Semantic category — what kind of knowledge this is |
| `subject` | Short label (3–8 words) — the routing key for display |
| `content` | Full, self-contained description — no assumed context |
| `scope` | `personal` · `project` · `global` · `temporary` |
| `importance` | Retrieval priority: 0.9+ = critical · 0.7 = useful · 0.5 = default · 0.3 = minor |
| `confidence` | How trusted: 0.95 = verified · 0.8 = likely · 0.6 = uncertain |
| `status` | `active` (default) · `superseded` · `stale` |
| `metadata.project` | Workspace routing key — the workspace_path this belongs to |
| `metadata.topic` | Domain branch (e.g. "auth", "agent-memory", "deployment") |
| `metadata.tags` | Flexible labels for cross-cutting retrieval |
| `metadata.source` | Always `"claude-code"` |
| `metadata.created_from` | `"conversation"` · `"observation"` · `"correction"` |

Inactive memories (`status: superseded` or `stale`) are excluded from all queries by default. Use `include_inactive: true` only when auditing history.

---

## Phase 1: Session Start — Load Context

Identify `<workspace_path>` (the current working directory Claude Code is running in).

### Track A — Global (always load, no project filter)

These are cross-project: how you like things done, reusable patterns.

```
memory.get_preferences(topic="coding style tools workflow communication")
memory.get_preferences(topic="output format verbosity explanations")
memory.search(
  query="cross-project reusable patterns constraints identity",
  filters={"scope": "global"},
  limit=5
)
```

Apply preferences silently — never list them unless the user asks.

### Track B — Project (load when workspace is known)

```
memory.get_project_context(project=<workspace_path>)
memory.get_decisions(project=<workspace_path>, limit=10)
memory.recent(project=<workspace_path>, limit=10)
memory.search(
  query="last session summary <project_folder_name>",
  filters={"memory_type": "conversation_summary", "project": <workspace_path>},
  limit=1
)
```

### Load constraints

- Load at most 25–30 memories total
- Prefer high-importance + recent items when trimming
- After loading, say **one line**: "I have context on [X]. Last session: [Y]."
- If nothing relevant found, say nothing and proceed

---

## Phase 2: Pre-Task Retrieval

Before certain types of work, retrieve type-specific memories:

| Situation | Retrieve |
|---|---|
| Starting planning or design | `memory.get_preferences(topic="architecture planning approach")` |
| Before architecture or tech choices | `memory.get_decisions(project=<workspace_path>)` |
| Before debugging | `memory.search(query="<error or symptom>", filters={"memory_type": "problem"})` then `filters={"memory_type": "solution"}` |
| Before implementing a feature | `memory.search(query="constraints for <feature>", filters={"memory_type": "constraint", "project": <workspace_path>})` |
| User references prior work | `memory.search(query="<what they referenced>", filters={"project": <workspace_path>})` |

---

## Phase 3: During Work — Capture

Write a memory immediately when you detect these signals. Do not batch — write on detection.

| Signal | Type | Scope |
|---|---|---|
| "I prefer…", "always use…", "I like/hate…" | `preference` | `personal` |
| Confirmed architectural or design choice | `decision` | `project` |
| Bug, error, or recurring pain | `problem` | `project` |
| Fix, workaround, command that worked | `solution` | `project` |
| Hard limit (budget, infra, policy, legal) | `constraint` | `project` |
| Pending task or follow-up | `task` | `project` |
| File, repo, link, or generated output | `artifact` | `project` |
| "Actually that was wrong" / "use X not Y" | `correction` | `personal` |
| Stable fact about environment/setup | `profile_fact` | `personal` |
| Project goals, scope, tech stack | `project_context` | `project` |
| Reusable pattern across projects | `learning` | `global` |

**Write format:**

```json
{
  "type": "<memory_type>",
  "content": "<full, self-contained statement — no pronouns, no assumed context>",
  "metadata": {
    "subject": "<short label, 3–8 words>",
    "importance": 0.7,
    "confidence": 0.8,
    "scope": "<personal|project|global|temporary>",
    "project": "<workspace_path>",
    "workspace_path": "<workspace_path>",
    "topic": "<domain branch, e.g. auth, deployment, memory>",
    "tags": ["<tag1>", "<tag2>"],
    "source": "claude-code",
    "created_from": "conversation",
    "status": "active"
  }
}
```

**Write principles:**
- `content` must stand alone — future Claude reads it with zero context from this session
- Omit `project`/`workspace_path` for `personal`/`global` scope memories
- Do not write trivial facts, one-off requests, or things already in the codebase
- When a correction supersedes an earlier memory, note it: write the correction, and mark the old pattern in context as superseded

---

## Phase 4: Session End — Summarize

When substantial work is complete, write a summary:

```json
{
  "type": "conversation_summary",
  "content": "Session in <project_folder_name> (<date>): worked on <topic>. Decisions made: <list>. Problems encountered: <list>. Solutions found: <list>. Open questions: <list>. Next steps: <list>.",
  "metadata": {
    "subject": "<project_name>: <topic> session",
    "importance": 0.6,
    "confidence": 1.0,
    "scope": "project",
    "project": "<workspace_path>",
    "workspace_path": "<workspace_path>",
    "topic": "<main topic>",
    "tags": ["session-summary"],
    "source": "claude-code",
    "created_from": "conversation",
    "status": "active"
  }
}
```

Also write individually any unresolved items:

```json
{"type": "question", "content": "...", "metadata": {"subject": "...", "project": "...", "scope": "project", "status": "active", "source": "claude-code"}}
{"type": "task",     "content": "...", "metadata": {"subject": "...", "project": "...", "scope": "project", "status": "active", "source": "claude-code"}}
```

---

## Available Tools

| Tool | Signature | Use for |
|---|---|---|
| `memory.search` | `query, filters?, limit, include_inactive` | Broad semantic search across all types |
| `memory.write` | `type, content, metadata` | Write any memory event |
| `memory.get_preferences` | `topic, limit, include_inactive` | Load user preferences by topic |
| `memory.get_decisions` | `project, limit, include_inactive` | Load project decisions |
| `memory.get_project_context` | `project, limit, include_inactive` | Load project architecture/context |
| `memory.recent` | `project, limit, include_inactive` | Recent active memories for a workspace |

`filters` for `memory.search`: `{ memory_type?, scope?, project? }`
