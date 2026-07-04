-- D1 schema for the embedded OpenID Provider (zitadel/oidc).
-- Durable state only; short-lived state (auth requests, codes, access tokens)
-- lives in Cloudflare KV with TTL.
-- Apply with: wrangler d1 execute manga --file=migrations/0002_oidc.sql

-- OAuth2 clients (statically seeded + dynamically registered via DCR).
CREATE TABLE IF NOT EXISTS oidc_client (
  id                        TEXT PRIMARY KEY,
  secret_hash               TEXT,               -- bcrypt/argon2; empty for public (PKCE) clients
  application_type          INTEGER DEFAULT 0,  -- 0=web 1=native 2=user-agent
  auth_method               TEXT DEFAULT 'none',
  redirect_uris             TEXT,               -- JSON array
  post_logout_redirect_uris TEXT,               -- JSON array
  grant_types               TEXT,               -- JSON array
  response_types            TEXT,               -- JSON array
  scopes                    TEXT,               -- JSON array
  access_token_type         INTEGER DEFAULT 0,  -- 0=bearer(JWT) 1=JWT
  dev_mode                  INTEGER DEFAULT 0,
  client_name               TEXT,
  registration_access_token TEXT,               -- RFC 7592 management token
  created_at                INTEGER DEFAULT 0
);

-- Signing keys (RSA/EC). One active key is used to sign tokens; kept so tokens
-- survive container restarts.
CREATE TABLE IF NOT EXISTS oidc_signing_key (
  id          TEXT PRIMARY KEY,
  algorithm   TEXT NOT NULL,       -- e.g. RS256
  private_key TEXT NOT NULL,       -- PEM (PKCS8)
  active      INTEGER DEFAULT 1,
  created_at  INTEGER DEFAULT 0
);

-- Refresh tokens (durable so admin sessions survive restart/sleep).
CREATE TABLE IF NOT EXISTS oidc_refresh_token (
  id         TEXT PRIMARY KEY,     -- token ID
  token      TEXT NOT NULL,        -- the actual refresh token value handed to the client
  client_id  TEXT NOT NULL,
  user_id    TEXT NOT NULL,        -- subject
  scopes     TEXT,                 -- JSON array
  audience   TEXT,                 -- JSON array
  amr        TEXT,                 -- JSON array
  auth_time  INTEGER DEFAULT 0,
  expiration INTEGER DEFAULT 0,
  created_at INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_refresh_token ON oidc_refresh_token(token);
CREATE INDEX IF NOT EXISTS idx_refresh_user  ON oidc_refresh_token(user_id, client_id);

-- Admin users (email + password) for the TanStack admin page login.
CREATE TABLE IF NOT EXISTS admin_user (
  id            TEXT PRIMARY KEY,
  email         TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,      -- argon2id
  name          TEXT,
  email_verified INTEGER DEFAULT 1,
  created_at    INTEGER DEFAULT 0
);
