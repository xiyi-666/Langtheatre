CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  nickname TEXT,
  avatar_url TEXT,
  bio TEXT,
  total_xp INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT;

CREATE TABLE IF NOT EXISTS theaters (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  language TEXT NOT NULL,
  topic TEXT NOT NULL,
  difficulty NUMERIC(3,1) NOT NULL,
  mode TEXT NOT NULL,
  status TEXT NOT NULL,
  is_favorite BOOLEAN NOT NULL DEFAULT false,
  share_code TEXT NOT NULL DEFAULT '',
  scene_description TEXT NOT NULL DEFAULT '',
  characters JSONB NOT NULL DEFAULT '[]'::jsonb,
  dialogues JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE theaters ADD COLUMN IF NOT EXISTS is_favorite BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE theaters ADD COLUMN IF NOT EXISTS share_code TEXT NOT NULL DEFAULT '';
ALTER TABLE theaters ADD COLUMN IF NOT EXISTS scene_description TEXT NOT NULL DEFAULT '';
ALTER TABLE theaters ADD COLUMN IF NOT EXISTS characters JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE theaters ADD COLUMN IF NOT EXISTS quiz_questions JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_theater_user_language ON theaters(user_id, language);

CREATE TABLE IF NOT EXISTS practice_records (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  theater_id UUID NOT NULL REFERENCES theaters(id),
  score INT NOT NULL,
  answers JSONB NOT NULL DEFAULT '[]'::jsonb,
  duration_seconds INT NOT NULL DEFAULT 0,
  xp_earned INT NOT NULL DEFAULT 0,
  completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_record_user_theater ON practice_records(user_id, theater_id);

CREATE TABLE IF NOT EXISTS courses (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  language TEXT NOT NULL,
  category TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  min_level NUMERIC(3,1) NOT NULL DEFAULT 4.0,
  max_level NUMERIC(3,1) NOT NULL DEFAULT 8.0,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roleplay_sessions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  theater_id UUID NOT NULL REFERENCES theaters(id),
  user_role TEXT NOT NULL,
  turn_index INT NOT NULL DEFAULT 0,
  current_score INT NOT NULL DEFAULT 0,
  transcript JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL DEFAULT 'active',
  final_feedback TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_roleplay_user ON roleplay_sessions(user_id, status);

INSERT INTO courses(language, category, title, description, min_level, max_level, is_active)
SELECT 'CANTONESE', 'daily', '茶餐厅点单', '学习香港茶餐厅常见点单表达', 4.0, 6.0, true
WHERE NOT EXISTS (
  SELECT 1 FROM courses WHERE language = 'CANTONESE' AND title = '茶餐厅点单'
);

INSERT INTO courses(language, category, title, description, min_level, max_level, is_active)
SELECT 'ENGLISH', 'ielts', 'Describe a memorable journey', 'IELTS speaking high-frequency topic', 5.5, 8.0, true
WHERE NOT EXISTS (
  SELECT 1 FROM courses WHERE language = 'ENGLISH' AND title = 'Describe a memorable journey'
);
