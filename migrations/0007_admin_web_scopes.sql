-- Reconcile the admin-web client's granted scopes. seedPublicClient (seed.go)
-- only INSERTs a client when it is absent, so an already-seeded admin-web row
-- does not pick up scopes added later (connections.write, users.read/write,
-- admin.read/write, and the per-kind taksonomi.*.write). Fresh installs get the
-- full set from the seed; this migration brings existing rows in line.
--
-- Idempotent: it sets an absolute value, so re-running is a no-op. Keep this
-- list in sync with seedAdminClient in internal/oidc/seed.go.
UPDATE oidc_client
SET scopes = '["openid","profile","email","offline_access","manga.write","manga.read","connections.write","users.read","users.write","admin.read","admin.write","taksonomi.genres.write","taksonomi.categories.write","taksonomi.authors.write","taksonomi.artists.write"]'
WHERE id = 'admin-web';
