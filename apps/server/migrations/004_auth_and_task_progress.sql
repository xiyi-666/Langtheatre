-- Multi-account authentication. Existing accounts remain usable and receive a stable legacy username.
ALTER TABLE users ADD COLUMN IF NOT EXISTS username TEXT;
UPDATE users
SET username = 'legacy_' || REPLACE(SUBSTRING(id::text FROM 1 FOR 8), '-', '')
WHERE username IS NULL OR BTRIM(username) = '';
ALTER TABLE users ALTER COLUMN username SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_ci ON users (LOWER(username));

-- 001_init.sql made email unique. Remove that legacy constraint so one email can
-- own up to three separately identified accounts (enforced by the trigger below).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;

-- Existing accounts predate verification; newly created accounts must verify their email.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN;
UPDATE users SET email_verified = TRUE WHERE email_verified IS NULL;
ALTER TABLE users ALTER COLUMN email_verified SET DEFAULT FALSE;
ALTER TABLE users ALTER COLUMN email_verified SET NOT NULL;

ALTER TABLE theaters ADD COLUMN IF NOT EXISTS generation_progress INT NOT NULL DEFAULT 0;
ALTER TABLE theaters ADD COLUMN IF NOT EXISTS generation_message TEXT NOT NULL DEFAULT '';
ALTER TABLE reading_materials ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'READY';
ALTER TABLE reading_materials ADD COLUMN IF NOT EXISTS generation_progress INT NOT NULL DEFAULT 100;
ALTER TABLE reading_materials ADD COLUMN IF NOT EXISTS generation_message TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS auth_tokens (
  token_hash TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_purpose ON auth_tokens(user_id, purpose);

CREATE OR REPLACE FUNCTION enforce_email_account_limit() RETURNS trigger AS $$
BEGIN
  PERFORM pg_advisory_xact_lock(hashtext(LOWER(NEW.email)));
  IF (SELECT COUNT(*) FROM users WHERE LOWER(email) = LOWER(NEW.email)) >= 3 THEN
    RAISE EXCEPTION 'email account limit reached';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS users_email_account_limit ON users;
CREATE TRIGGER users_email_account_limit
BEFORE INSERT ON users
FOR EACH ROW EXECUTE FUNCTION enforce_email_account_limit();
