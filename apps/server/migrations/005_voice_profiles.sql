CREATE TABLE IF NOT EXISTS voice_profiles (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  prompt TEXT NOT NULL,
  language TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT 'XIAOMI',
  model TEXT NOT NULL DEFAULT 'mimo-v2.5-tts-voicedesign',
  preview_audio_url TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'GENERATING',
  generation_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_voice_profiles_user_created
  ON voice_profiles(user_id, created_at DESC);
