package domain

// CoverMirrorJob is the queue message to mirror a media's external cover image
// into R2 as AVIF. It is enqueued on media create/update when the cover is an
// external URL; the consumer fetches the image, encodes it to AVIF, uploads it
// to R2, and rewrites media.cover_url to the stored R2 key.
type CoverMirrorJob struct {
	MediaID   string `json:"mediaId"`
	SourceURL string `json:"sourceUrl"`
}
