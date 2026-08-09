CREATE OR REPLACE FUNCTION sync_writing_prompt_minutes()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  instructions TEXT;
  minutes_text TEXT;
BEGIN
  IF NEW.prompt IS NULL
     OR jsonb_typeof(NEW.prompt) <> 'object'
     OR jsonb_typeof(NEW.prompt -> 'Instructions') IS DISTINCT FROM 'string'
     OR NEW.time_limit_seconds IS NULL THEN
    RETURN NEW;
  END IF;

  instructions := NEW.prompt ->> 'Instructions';
  minutes_text := CEIL(NEW.time_limit_seconds / 60.0)::bigint::text;
  instructions := regexp_replace(
    instructions,
    'You should spend about [0-9]+ minutes on this task\.',
    'You should spend about ' || minutes_text || ' minutes on this task.',
    'g'
  );
  NEW.prompt := jsonb_set(NEW.prompt, '{Instructions}', to_jsonb(instructions), false);
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_sync_writing_prompt_minutes ON writing_sessions;
CREATE TRIGGER trg_sync_writing_prompt_minutes
BEFORE INSERT OR UPDATE ON writing_sessions
FOR EACH ROW
EXECUTE FUNCTION sync_writing_prompt_minutes();

UPDATE reading_materials
SET audio_status = 'READY'
WHERE status = 'READY'
  AND audio_status = 'PENDING';
