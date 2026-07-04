-- External-source OAuth connections (MyAnimeList first; IMDB later).
-- client_secret / access_token / refresh_token hold AES-GCM ciphertext at rest
-- (see CONNECTIONS_ENC_KEY). Apply with:
--   wrangler d1 execute manga --file=migrations/0005_connections.sql

CREATE TABLE IF NOT EXISTS connection (
    id            TEXT PRIMARY KEY,
    provider      TEXT NOT NULL,
    label         TEXT,
    client_id     TEXT,
    client_secret TEXT,
    access_token  TEXT,
    refresh_token TEXT,
    token_type    TEXT,
    expires_at    INTEGER,
    status        TEXT NOT NULL DEFAULT 'disconnected',
    created_at    INTEGER,
    updated_at    INTEGER
);

CREATE INDEX IF NOT EXISTS idx_connection_provider ON connection(provider);
