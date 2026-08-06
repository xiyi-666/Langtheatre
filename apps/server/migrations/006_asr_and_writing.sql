CREATE TABLE IF NOT EXISTS asr_configs (
  id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  base_url TEXT NOT NULL,
  api_key TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE roleplay_sessions ADD COLUMN IF NOT EXISTS processing_message TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS writing_sessions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  exam TEXT NOT NULL,
  time_limit_seconds INT NOT NULL,
  prompt JSONB NOT NULL,
  essay TEXT NOT NULL DEFAULT '',
  word_count INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  progress_message TEXT NOT NULL DEFAULT '',
  evaluation JSONB,
  started_at TIMESTAMPTZ NOT NULL,
  submitted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_writing_sessions_user_created ON writing_sessions(user_id, created_at DESC);
