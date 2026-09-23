CREATE TABLE IF NOT EXISTS speaking_sessions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  part INT NOT NULL DEFAULT 1,
  prompt_index INT NOT NULL DEFAULT 0,
  preparation_ends_at TIMESTAMPTZ,
  answer_ends_at TIMESTAMPTZ,
  prompts JSONB NOT NULL DEFAULT '[]'::jsonb,
  turns JSONB NOT NULL DEFAULT '[]'::jsonb,
  evaluation JSONB,
  processing_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_speaking_sessions_user_created ON speaking_sessions(user_id, created_at DESC);
