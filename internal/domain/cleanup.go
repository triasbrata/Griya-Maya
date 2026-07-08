package domain

// PageCleanupJob is the queue message to delete orphaned R2 objects after their
// D1 rows have been removed. It is enqueued when a chapter, a single chapter
// page, or a whole media entry is deleted; the consumer batch-deletes the keys
// from R2 (a "not found" object is treated as already-cleaned).
//
// Keys are individual object keys (manga pages, ad creatives, covers). Prefixes
// are recursive: the consumer lists every object under each prefix and deletes
// it — used for HLS video bundles (hls/{id}/), where a chapter row records only
// the playlist key but the init + segment objects live beside it.
type PageCleanupJob struct {
	Keys     []string `json:"keys,omitempty"`
	Prefixes []string `json:"prefixes,omitempty"`
}
