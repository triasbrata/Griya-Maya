-- Normalize the content source into its own table. media.source_id has always
-- carried a source slug (e.g. "griyamedia"); this promotes it to a first-class,
-- managed entity so sources can be listed by the reader (manga.read) and managed
-- by admins (admin.read/admin.write). media.source_id stays as the logical
-- foreign key (SQLite/D1 does not enforce it, matching the rest of the schema).
CREATE TABLE IF NOT EXISTS source (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  lang       TEXT NOT NULL DEFAULT 'en',
  icon_url   TEXT NOT NULL DEFAULT '',
  enabled    INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0
);

-- Backfill: every distinct source_id already in the catalog becomes a source row
-- so existing media keep a valid parent. Name defaults to the id; edit later.
INSERT OR IGNORE INTO source (id, name, lang, icon_url, enabled, created_at, updated_at)
SELECT DISTINCT source_id, source_id, 'en', '', 1, strftime('%s','now'), strftime('%s','now')
FROM media WHERE source_id IS NOT NULL AND source_id <> '';

-- Ensure the canonical source exists even on a fresh/empty catalog.
INSERT OR IGNORE INTO source (id, name, lang, icon_url, enabled, created_at, updated_at)
VALUES ('griyamedia', 'GriyaMedia', 'en', '', 1, strftime('%s','now'), strftime('%s','now'));
