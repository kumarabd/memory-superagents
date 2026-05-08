CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE conversation_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_name text,
  workspace_path text,
  started_at timestamptz DEFAULT now(),
  ended_at timestamptz,
  summary text,
  metadata jsonb DEFAULT '{}'
);

CREATE TABLE messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id uuid REFERENCES conversation_sessions(id),
  role text,
  content text,
  created_at timestamptz DEFAULT now(),
  metadata jsonb DEFAULT '{}'
);

CREATE TABLE memory_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id uuid REFERENCES conversation_sessions(id),
  memory_type text CHECK (
    memory_type IN (
      'preference',
      'profile_fact',
      'project_context',
      'decision',
      'task',
      'event',
      'problem',
      'solution',
      'learning',
      'question',
      'plan',
      'constraint',
      'credential_reference',
      'relationship',
      'routine',
      'artifact',
      'conversation_summary',
      'correction',
      'feedback',
      'observation',
      'hypothesis',
      'experiment',
      'capability',
      'policy',
      'identity'
    )
  ),
  subject text,
  content text,
  importance float DEFAULT 0.5,
  confidence float DEFAULT 0.7,
  scope text DEFAULT 'personal',
  created_at timestamptz DEFAULT now(),
  metadata jsonb DEFAULT '{}',
  embedding vector(1536)
);
