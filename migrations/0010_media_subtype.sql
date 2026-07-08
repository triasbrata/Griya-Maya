-- 0010_media_subtype.sql
-- Replace the freeform "genre" taxonomy with a single, type-scoped sub_type
-- classifier stored directly on the media row. A media's sub_type is one of a
-- fixed vocabulary bound to its type (manga: manga|manhwa|manhua, novel:
-- web_novel|light_novel, video: anime_movie|anime_series|tv_series). The genre
-- table + its join are dropped; category/author/artist taxonomies are untouched.
-- Forward-only.

-- 1) New first-class column. SQLite/D1 ALTER ADD COLUMN with a constant default
--    backfills existing rows to '' (sub_type is optional).
ALTER TABLE media ADD COLUMN sub_type TEXT NOT NULL DEFAULT '';

-- 2) Pragmatic backfill: existing manga default to the "manga" sub_type (the
--    dominant kind and a valid slug for the manga type). novel/video are left
--    blank for operators to set explicitly, since their sub_types are ambiguous.
UPDATE media SET sub_type = 'manga' WHERE type = 'manga' AND sub_type = '';

-- 3) Filter/index parity with the type column (source-scoped browse filter).
CREATE INDEX IF NOT EXISTS idx_media_subtype ON media(source_id, sub_type);

-- 4) Drop the retired genre taxonomy (join first, then the tag table).
DROP INDEX IF EXISTS idx_media_genre_genre;
DROP TABLE IF EXISTS media_genre;
DROP TABLE IF EXISTS genre;
