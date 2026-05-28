CREATE TABLE IF NOT EXISTS tts_configs (
  id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  provider TEXT NOT NULL DEFAULT 'XIAOMI',
  model TEXT NOT NULL DEFAULT 'mimo-v2.5-tts',
  base_url TEXT NOT NULL DEFAULT 'https://token-plan-cn.xiaomimimo.com/v1',
  api_key TEXT NOT NULL DEFAULT '',
  voice TEXT NOT NULL DEFAULT 'mimo_default',
  audio_format TEXT NOT NULL DEFAULT 'wav',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
