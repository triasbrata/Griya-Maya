-- 0004_media_normalize.sql
-- Generalize the manga-only catalog into a unified `media` entity (manga | video
-- | novel) and normalize the denormalized taxonomy columns (genres/author/artist)
-- into their own tables + join tables.
--
-- Forward-only. Apply once with:
--   wrangler d1 execute manga --remote --file=migrations/0004_media_normalize.sql
--
-- ORDERING IS LOAD-BEARING:
--   (A) rename table + columns + indexes
--   (B) create taxonomy + join tables
--   (C) back-fill from the legacy denormalized columns
--   (D) DROP the legacy columns  <-- must be LAST, after back-fill reads them
--
-- Idempotency: sections B and C use IF NOT EXISTS / INSERT OR IGNORE and are
-- re-runnable. Sections A and D are NOT idempotent (ALTER ... RENAME / DROP
-- COLUMN fail on a second run). `wrangler d1 execute --file` runs statements
-- sequentially and is NOT one remote transaction: if the file fails midway, fix
-- and hand-run the remaining statements; do not blindly re-execute the whole file.

------------------------------------------------------------------------
-- (A) manga -> media, add type, rename FK/reference columns, reindex
------------------------------------------------------------------------
ALTER TABLE manga RENAME TO media;
ALTER TABLE media ADD COLUMN type TEXT NOT NULL DEFAULT 'manga';   -- manga | video | novel

-- chapter.manga_id carries a real FOREIGN KEY(...) REFERENCES manga(id). With
-- SQLite's default PRAGMA legacy_alter_table=OFF (D1's default):
--   1. RENAME TABLE manga->media auto-rewrites chapter's FK target to media(id).
--   2. RENAME COLUMN manga_id->media_id auto-rewrites the FK's local column name.
-- So no manual FK surgery / table rebuild is needed.
ALTER TABLE chapter RENAME COLUMN manga_id TO media_id;

-- convert_job.manga_id is a plain column (no FK) -> straight rename.
ALTER TABLE convert_job RENAME COLUMN manga_id TO media_id;

-- Recreate indexes under media_* names for consistency.
DROP INDEX IF EXISTS idx_manga_source;
DROP INDEX IF EXISTS idx_manga_title;
DROP INDEX IF EXISTS idx_chapter_manga;
CREATE INDEX IF NOT EXISTS idx_media_source  ON media(source_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_title   ON media(source_id, title);
CREATE INDEX IF NOT EXISTS idx_chapter_media ON chapter(media_id, number);

------------------------------------------------------------------------
-- (B) taxonomy + join tables
-- id = opaque lower(hex(randomblob(16))); dedup enforced by UNIQUE(slug) for
-- genre/category and UNIQUE(name) for author/artist.
------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS genre    (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS category (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS author   (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS artist   (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE);

CREATE TABLE IF NOT EXISTS media_genre (
  media_id TEXT NOT NULL,
  genre_id TEXT NOT NULL,
  PRIMARY KEY (media_id, genre_id),
  FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
  FOREIGN KEY (genre_id) REFERENCES genre(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_media_genre_genre ON media_genre(genre_id);

CREATE TABLE IF NOT EXISTS media_category (
  media_id    TEXT NOT NULL,
  category_id TEXT NOT NULL,
  PRIMARY KEY (media_id, category_id),
  FOREIGN KEY (media_id)    REFERENCES media(id)    ON DELETE CASCADE,
  FOREIGN KEY (category_id) REFERENCES category(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_media_category_category ON media_category(category_id);

CREATE TABLE IF NOT EXISTS media_author (
  media_id  TEXT NOT NULL,
  author_id TEXT NOT NULL,
  PRIMARY KEY (media_id, author_id),
  FOREIGN KEY (media_id)  REFERENCES media(id)  ON DELETE CASCADE,
  FOREIGN KEY (author_id) REFERENCES author(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_media_author_author ON media_author(author_id);

CREATE TABLE IF NOT EXISTS media_artist (
  media_id  TEXT NOT NULL,
  artist_id TEXT NOT NULL,
  PRIMARY KEY (media_id, artist_id),
  FOREIGN KEY (media_id)  REFERENCES media(id)  ON DELETE CASCADE,
  FOREIGN KEY (artist_id) REFERENCES artist(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_media_artist_artist ON media_artist(artist_id);

------------------------------------------------------------------------
-- (C) BACK-FILL from the legacy denormalized columns
------------------------------------------------------------------------

-- (C1) genres: split the comma-separated media.genres via a recursive CTE.
-- slug = lower(name) with spaces -> hyphens, matching Go genreSlug().
INSERT OR IGNORE INTO genre (id, slug, name)
WITH RECURSIVE split(media_id, remain, part) AS (
  SELECT id, genres || ',', ''
  FROM media
  WHERE genres IS NOT NULL AND genres <> ''
  UNION ALL
  SELECT media_id,
         substr(remain, instr(remain, ',') + 1),
         substr(remain, 1, instr(remain, ',') - 1)
  FROM split
  WHERE remain <> ''
)
SELECT lower(hex(randomblob(16))),
       lower(replace(trim(part), ' ', '-')),
       trim(part)
FROM split
WHERE trim(part) <> ''
GROUP BY lower(replace(trim(part), ' ', '-'));

-- (C2) media_genre links: re-split, join each token to its genre by slug.
INSERT OR IGNORE INTO media_genre (media_id, genre_id)
WITH RECURSIVE split(media_id, remain, part) AS (
  SELECT id, genres || ',', ''
  FROM media
  WHERE genres IS NOT NULL AND genres <> ''
  UNION ALL
  SELECT media_id,
         substr(remain, instr(remain, ',') + 1),
         substr(remain, 1, instr(remain, ',') - 1)
  FROM split
  WHERE remain <> ''
)
SELECT s.media_id, g.id
FROM split s
JOIN genre g ON g.slug = lower(replace(trim(s.part), ' ', '-'))
WHERE trim(s.part) <> '';

-- (C3) authors: single free-text column -> one author per row (no split).
INSERT OR IGNORE INTO author (id, name)
SELECT lower(hex(randomblob(16))), trim(author)
FROM media
WHERE author IS NOT NULL AND trim(author) <> ''
GROUP BY trim(author);

INSERT OR IGNORE INTO media_author (media_id, author_id)
SELECT m.id, a.id
FROM media m
JOIN author a ON a.name = trim(m.author)
WHERE m.author IS NOT NULL AND trim(m.author) <> '';

-- (C4) artists: same shape as authors.
INSERT OR IGNORE INTO artist (id, name)
SELECT lower(hex(randomblob(16))), trim(artist)
FROM media
WHERE artist IS NOT NULL AND trim(artist) <> ''
GROUP BY trim(artist);

INSERT OR IGNORE INTO media_artist (media_id, artist_id)
SELECT m.id, a.id
FROM media m
JOIN artist a ON a.name = trim(m.artist)
WHERE m.artist IS NOT NULL AND trim(m.artist) <> '';

-- NOTE: media_category is intentionally left empty — the legacy schema has no
-- category source data to back-fill. Categories populate going forward via the
-- management endpoints / ingestion.

------------------------------------------------------------------------
-- (D) drop legacy denormalized columns (LAST — section C reads them)
------------------------------------------------------------------------
ALTER TABLE media DROP COLUMN genres;
ALTER TABLE media DROP COLUMN author;
ALTER TABLE media DROP COLUMN artist;
