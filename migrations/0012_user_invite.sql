-- Registration invites: gate self-service signup so account creation is not
-- fully open. A person can only register via POST /v1/register by presenting a
-- valid, unused, unexpired invite code. An invite may be bound to a specific
-- email (allowlist entry) or left open (email = '' / NULL means any email).
--
-- New accounts created through registration land unverified (email_verified = 0)
-- and cannot log in until an admin verifies them (PUT /v1/users/{id}).
CREATE TABLE IF NOT EXISTS user_invite (
  code       TEXT PRIMARY KEY,       -- opaque random token presented at signup
  email      TEXT,                   -- '' / NULL = any email may use this invite
  note       TEXT,                   -- admin-facing label (who/why it was issued)
  expires_at INTEGER DEFAULT 0,      -- unix seconds; 0 = never expires
  used_at    INTEGER DEFAULT 0,      -- unix seconds it was consumed; 0 = unused
  used_by    TEXT,                   -- admin_user.id created from this invite
  created_at INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_user_invite_email ON user_invite(email);
