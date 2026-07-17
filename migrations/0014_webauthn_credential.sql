-- WebAuthn/passkey credentials for biometric login (FIDO2).
-- One row per registered authenticator. A user may hold several (phone, laptop,
-- security key). The full go-webauthn Credential record is stored as JSON in
-- `data` (opaque to SQL); `id` (base64url of the credential ID) is the lookup
-- key used when an authenticator asserts during login.
-- Apply with: wrangler d1 execute manga --file=migrations/0014_webauthn_credential.sql

CREATE TABLE IF NOT EXISTS webauthn_credential (
  id           TEXT PRIMARY KEY,     -- base64url(credential.ID)
  user_id      TEXT NOT NULL,        -- admin_user.id (also the WebAuthn user handle)
  name         TEXT,                 -- human label for the device (optional)
  data         TEXT NOT NULL,        -- JSON of go-webauthn Credential (public key, sign count, ...)
  created_at   INTEGER DEFAULT 0,
  last_used_at INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_webauthn_cred_user ON webauthn_credential(user_id);
