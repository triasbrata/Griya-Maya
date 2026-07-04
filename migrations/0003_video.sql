-- HLS video support: a page can be an image (default) or an HLS playlist.
-- Apply with: wrangler d1 execute manga --file=migrations/0003_video.sql

ALTER TABLE page ADD COLUMN kind TEXT NOT NULL DEFAULT 'image';  -- image | video
