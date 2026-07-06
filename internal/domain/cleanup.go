package domain

// PageCleanupJob is the queue message to delete orphaned R2 objects after their
// D1 rows have been removed. It is enqueued when a chapter, a single chapter
// page, or a whole media entry is deleted; the consumer batch-deletes the keys
// from R2 (a "not found" object is treated as already-cleaned).
type PageCleanupJob struct {
	Keys []string `json:"keys"`
}
