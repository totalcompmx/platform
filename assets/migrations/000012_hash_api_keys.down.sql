-- Hashing is one-way: existing hashed keys cannot be restored to plaintext.
-- Users would need to regenerate their key after rolling back.
ALTER TABLE users DROP COLUMN IF EXISTS api_key_prefix;

COMMENT ON COLUMN users.api_key IS 'Hashed API key for stateless API authentication';
