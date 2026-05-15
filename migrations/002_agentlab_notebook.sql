-- AgentLab structured notebook (moved out of .agentlab/context.json).
-- workspace_key: absolute project path (same convention as memory_events metadata.project).

CREATE TABLE IF NOT EXISTS agentlab_notebook (
  workspace_key text PRIMARY KEY,
  payload jsonb NOT NULL DEFAULT '{}',
  version bigint NOT NULL DEFAULT 1,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS agentlab_notebook_updated_at
  ON agentlab_notebook (updated_at DESC);
