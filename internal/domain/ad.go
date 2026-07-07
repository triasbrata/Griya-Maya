package domain

import "time"

// Ad is a house-ad creative as served to the reader. The iOS reader fetches a
// slice of these (GET /v1/ads?placement=…) and interleaves them between chapter
// pages. ImageURL is a short-lived presigned R2 URL the client fetches directly,
// mirroring how a StoredPage's R2 key becomes a readable Page.ImageURL. The raw
// R2 key is deliberately absent here (reader-facing); the admin surface exposes
// it via StoredAd.
type Ad struct {
	ID          string  `json:"id"`
	ImageURL    string  `json:"imageUrl"`
	ClickURL    string  `json:"clickUrl"`
	Weight      int     `json:"weight"`
	Placement   string  `json:"placement"`
	AspectRatio float64 `json:"aspectRatio,omitempty"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
}

// StoredAd is a house ad as persisted (the private R2 object key, not a fetchable
// URL). The service turns R2Key into a presigned Ad.ImageURL for the reader; the
// admin surface returns StoredAd directly so an operator can see the raw key and
// active flag. Mirrors the StoredPage → Page split.
type StoredAd struct {
	ID        string    `json:"id"`
	R2Key     string    `json:"r2Key"`
	ClickURL  string    `json:"clickUrl"`
	Weight    int       `json:"weight"`
	Placement string    `json:"placement"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

// AdWriteRequest is the admin create/update payload for a house ad. R2Key points
// at an already-uploaded creative (via the presigned PUT flow); Weight defaults
// to 1 and Active to true when omitted.
type AdWriteRequest struct {
	R2Key     string `json:"r2Key"`
	ClickURL  string `json:"clickUrl,omitempty"`
	Weight    int    `json:"weight,omitempty"`
	Placement string `json:"placement,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Active    *bool  `json:"active,omitempty"`
}
