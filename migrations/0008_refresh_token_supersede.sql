-- Refresh-token rotation reuse-tolerance (fixes the "login mulu" race).
--
-- Rotation previously hard-deleted the old row then inserted the new one, which
-- is not atomic: two near-simultaneous redemptions of the same refresh token
-- (the iOS app fires parallel requests that all 401 once the 1h access token
-- expires) let the winner delete the row while the loser then finds it gone and
-- returns invalid_grant -> the user is signed out.
--
-- Instead of deleting, a rotated token is now marked *superseded* (it points at
-- its successor and records when). A superseded token stays redeemable for a
-- short grace window: concurrent/retried redemptions receive the winner's
-- successor rather than an error. A redemption AFTER the grace window is treated
-- as reuse/theft and revokes the chain (RFC 6819 5.2.2.3 / OAuth 2.1).
--
-- Apply with: wrangler d1 execute manga --remote --file=migrations/0008_refresh_token_supersede.sql
ALTER TABLE oidc_refresh_token ADD COLUMN superseded_by TEXT;
ALTER TABLE oidc_refresh_token ADD COLUMN superseded_at INTEGER NOT NULL DEFAULT 0;
