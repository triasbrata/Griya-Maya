package domain

// VideoRegisterRequest associates an already-uploaded HLS bundle in R2 with a
// chapter, so the reader endpoint serves that chapter as a single video page.
//
// The bundle (an `index.m3u8` playlist plus its media segments) is uploaded
// first via POST /v1/video/upload; PlaylistKey is the returned R2 key of the
// `.m3u8`. Registration rewrites the chapter's pages to one PageKindVideo page
// pointing at that playlist.
type VideoRegisterRequest struct {
	// ChapterID is the catalog chapter these pages belong to (required).
	ChapterID string `json:"chapterId"`
	// PlaylistKey is the R2 object key of the HLS `.m3u8` playlist (required).
	PlaylistKey string `json:"playlistKey"`
	// Width / Height are the video's pixel dimensions (optional, informational).
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}
