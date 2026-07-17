-- Remembered OAuth consent grants: which scopes a user has already approved for
-- a given client. Lets the login flow skip the consent screen when a client
-- re-requests a scope set the user has already granted (see internal/oidc/consent.go).
-- Apply with: wrangler d1 execute manga --file=migrations/0013_oidc_consent.sql

CREATE TABLE IF NOT EXISTS oidc_user_consent (
  user_id    TEXT NOT NULL,        -- subject (admin_user.id)
  client_id  TEXT NOT NULL,        -- oidc_client.id
  scopes     TEXT,                 -- JSON array of granted scopes (union over time)
  created_at INTEGER DEFAULT 0,
  updated_at INTEGER DEFAULT 0,
  PRIMARY KEY (user_id, client_id)
);
