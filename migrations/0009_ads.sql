-- House ads: server-owner-supplied creatives the reader interleaves between
-- chapter pages. Creatives are stored in R2 (private) and served to the reader
-- (manga.read) as short-lived presigned URLs, just like page bytes. Ads are
-- managed by admins (admin.read/admin.write), mirroring the source table.
CREATE TABLE IF NOT EXISTS ads (
  id         TEXT PRIMARY KEY,
  r2_key     TEXT NOT NULL,
  click_url  TEXT NOT NULL DEFAULT '',
  weight     INTEGER NOT NULL DEFAULT 1,
  placement  TEXT NOT NULL DEFAULT '',
  width      INTEGER NOT NULL DEFAULT 0,
  height     INTEGER NOT NULL DEFAULT 0,
  active     INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL DEFAULT 0
);

-- The reader lists active ads for a placement, ordered by weight; index that path.
CREATE INDEX IF NOT EXISTS idx_ads_placement_active ON ads (placement, active);
