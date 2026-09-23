ALTER TABLE mock_exams
  ADD COLUMN IF NOT EXISTS evaluation_approval JSONB NOT NULL DEFAULT '{"status":"LEGACY_UNVERIFIED"}'::jsonb;

-- Existing results remain available to administrators in storage, but the
-- service will not expose them until an exact revision-bound review exists.
UPDATE mock_exams
SET evaluation_approval = '{"status":"LEGACY_UNVERIFIED"}'::jsonb
WHERE evaluation_approval IS NULL OR NOT (evaluation_approval ? 'status');
