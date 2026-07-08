-- 0011_genre_subtype_managed.sql
-- Reshape the media classification model:
--   (A) Re-introduce the multi-value managed "genre" taxonomy (retired in 0010),
--       exactly as 0004 shaped it (slug-bearing tag + media_genre join).
--   (B) Promote the sub_type vocabulary from a hardcoded Go map into a managed,
--       per-type `sub_type` table (seeded with the former fixed vocabulary).
--   (C) Drop the "category" taxonomy entirely (table + join + index). Category
--       filtering/listing is removed from the reader client.
--
-- Forward-only. Apply once with:
--   wrangler d1 execute manga --remote --file=migrations/0011_genre_subtype_managed.sql
--
-- Sections use IF NOT EXISTS / INSERT OR IGNORE and are re-runnable; the DROPs
-- are IF EXISTS. `wrangler d1 execute --file` runs statements sequentially and is
-- NOT one transaction: on a mid-file failure, fix and hand-run the remainder.

------------------------------------------------------------------------
-- (A) Re-create the genre taxonomy + its join (mirrors 0004 exactly).
------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS genre (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);

CREATE TABLE IF NOT EXISTS media_genre (
  media_id TEXT NOT NULL,
  genre_id TEXT NOT NULL,
  PRIMARY KEY (media_id, genre_id),
  FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
  FOREIGN KEY (genre_id) REFERENCES genre(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_media_genre_genre ON media_genre(genre_id);

------------------------------------------------------------------------
-- (B) Managed sub_type vocabulary. slug is the canonical wire value (PK);
--     type scopes it to manga|novel|video; name is the display label. Seed the
--     8 rows that were the former hardcoded `subTypesByType` vocabulary.
------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sub_type (
  slug TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  name TEXT NOT NULL
);

INSERT OR IGNORE INTO sub_type (slug, type, name) VALUES
  ('manga',        'manga', 'Manga'),
  ('manhwa',       'manga', 'Manhwa'),
  ('manhua',       'manga', 'Manhua'),
  ('web_novel',    'novel', 'Web Novel'),
  ('light_novel',  'novel', 'Light Novel'),
  ('anime_movie',  'video', 'Anime Movie'),
  ('anime_series', 'video', 'Anime Series'),
  ('tv_series',    'video', 'TV Series');

------------------------------------------------------------------------
-- (C) Drop the retired category taxonomy (join + index first, then the table).
------------------------------------------------------------------------
DROP INDEX IF EXISTS idx_media_category_category;
DROP TABLE IF EXISTS media_category;
DROP TABLE IF EXISTS category;
