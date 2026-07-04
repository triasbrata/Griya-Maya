-- D1 schema for the Mihon manga server.
-- Apply with: wrangler d1 execute manga --file=migrations/0001_init.sql

CREATE TABLE IF NOT EXISTS manga (
  id          TEXT PRIMARY KEY,
  source_id   TEXT NOT NULL,
  url         TEXT NOT NULL,
  title       TEXT NOT NULL,
  cover_url   TEXT,
  author      TEXT,
  artist      TEXT,
  description TEXT,
  genres      TEXT,                       -- comma-separated
  status      TEXT DEFAULT 'unknown',
  popularity  INTEGER DEFAULT 0,
  updated_at  INTEGER DEFAULT 0           -- unix seconds
);
CREATE INDEX IF NOT EXISTS idx_manga_source ON manga(source_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_manga_title  ON manga(source_id, title);

CREATE TABLE IF NOT EXISTS chapter (
  id          TEXT PRIMARY KEY,
  manga_id    TEXT NOT NULL,
  url         TEXT NOT NULL,
  name        TEXT NOT NULL,
  number      REAL DEFAULT 0,
  scanlator   TEXT,
  date_upload INTEGER DEFAULT 0,          -- unix seconds
  format      TEXT,                       -- cbz | epub | pdf | null
  FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chapter_manga ON chapter(manga_id, number);

CREATE TABLE IF NOT EXISTS page (
  chapter_id TEXT NOT NULL,
  idx        INTEGER NOT NULL,
  r2_key     TEXT NOT NULL,
  width      INTEGER DEFAULT 0,
  height     INTEGER DEFAULT 0,
  PRIMARY KEY (chapter_id, idx),
  FOREIGN KEY (chapter_id) REFERENCES chapter(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS convert_job (
  id            TEXT PRIMARY KEY,
  source_key    TEXT NOT NULL,
  format        TEXT,
  output_prefix TEXT,
  manga_id      TEXT,
  chapter_id    TEXT,
  status        TEXT NOT NULL DEFAULT 'pending',
  page_count    INTEGER DEFAULT 0,
  error         TEXT,
  created_at    INTEGER DEFAULT 0,
  updated_at    INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_job_status ON convert_job(status, updated_at DESC);
