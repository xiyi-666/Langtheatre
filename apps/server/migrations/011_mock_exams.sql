CREATE TABLE IF NOT EXISTS mock_exams (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  exam TEXT NOT NULL,
  status TEXT NOT NULL,
  current_section TEXT NOT NULL DEFAULT '',
  total_duration_seconds INT NOT NULL,
  sections JSONB NOT NULL DEFAULT '[]'::jsonb,
  result JSONB,
  started_at TIMESTAMPTZ NOT NULL,
  submitted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_mock_exams_user_created ON mock_exams(user_id, created_at DESC);
