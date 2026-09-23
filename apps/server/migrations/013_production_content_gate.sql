ALTER TABLE theaters
  ADD COLUMN IF NOT EXISTS production_approval JSONB NOT NULL DEFAULT '{"status":"LEGACY_UNVERIFIED"}'::jsonb;

ALTER TABLE reading_materials
  ADD COLUMN IF NOT EXISTS production_approval JSONB NOT NULL DEFAULT '{"status":"LEGACY_UNVERIFIED"}'::jsonb;

ALTER TABLE writing_sessions
  ADD COLUMN IF NOT EXISTS prompt_approval JSONB NOT NULL DEFAULT '{"status":"LEGACY_UNVERIFIED"}'::jsonb,
  ADD COLUMN IF NOT EXISTS evaluation_approval JSONB NOT NULL DEFAULT '{"status":"LEGACY_UNVERIFIED"}'::jsonb;

ALTER TABLE mock_exams
  ADD COLUMN IF NOT EXISTS production_approval JSONB NOT NULL DEFAULT '{"status":"LEGACY_UNVERIFIED"}'::jsonb,
  ADD COLUMN IF NOT EXISTS evaluation_approval JSONB NOT NULL DEFAULT '{"status":"LEGACY_UNVERIFIED"}'::jsonb;

ALTER TABLE speaking_sessions
  ADD COLUMN IF NOT EXISTS evaluation_approval JSONB NOT NULL DEFAULT '{"status":"LEGACY_UNVERIFIED"}'::jsonb;

-- Historical records are retained exactly as-is. Missing or malformed review
-- metadata is quarantined and can only be replaced by an explicit review.
UPDATE theaters
SET production_approval = '{"status":"LEGACY_UNVERIFIED"}'::jsonb
WHERE production_approval IS NULL OR NOT (production_approval ? 'status');

UPDATE reading_materials
SET production_approval = '{"status":"LEGACY_UNVERIFIED"}'::jsonb
WHERE production_approval IS NULL OR NOT (production_approval ? 'status');

UPDATE writing_sessions
SET prompt_approval = '{"status":"LEGACY_UNVERIFIED"}'::jsonb
WHERE prompt_approval IS NULL OR NOT (prompt_approval ? 'status');

UPDATE writing_sessions
SET evaluation_approval = '{"status":"LEGACY_UNVERIFIED"}'::jsonb
WHERE evaluation_approval IS NULL OR NOT (evaluation_approval ? 'status');

UPDATE mock_exams
SET production_approval = '{"status":"LEGACY_UNVERIFIED"}'::jsonb
WHERE production_approval IS NULL OR NOT (production_approval ? 'status');

UPDATE mock_exams
SET evaluation_approval = '{"status":"LEGACY_UNVERIFIED"}'::jsonb
WHERE evaluation_approval IS NULL OR NOT (evaluation_approval ? 'status');

UPDATE speaking_sessions
SET evaluation_approval = '{"status":"LEGACY_UNVERIFIED"}'::jsonb
WHERE evaluation_approval IS NULL OR NOT (evaluation_approval ? 'status');
