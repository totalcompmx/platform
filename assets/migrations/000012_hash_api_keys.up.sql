-- API keys move from plaintext to SHA-256 hashes. A short prefix of the
-- original key is kept so the dashboard can show which key is configured.
ALTER TABLE users ADD COLUMN api_key_prefix TEXT;

-- Existing keys were stored in plaintext; hash them in place so they keep
-- working (sha256() is built into PostgreSQL 11+).
UPDATE users
SET api_key_prefix = LEFT(api_key, 8),
    api_key = encode(sha256(convert_to(api_key, 'UTF8')), 'hex')
WHERE api_key IS NOT NULL;

COMMENT ON COLUMN users.api_key IS 'SHA-256 hex digest of the API key (plaintext keys are never stored)';
COMMENT ON COLUMN users.api_key_prefix IS 'Leading characters of the API key, for display in the dashboard';
