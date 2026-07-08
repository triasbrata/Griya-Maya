package domain

import (
	"path"
	"strings"
)

// VideoPresignFile names one file the client intends to upload straight to R2.
// ContentType is optional; when empty the server derives it from the extension.
type VideoPresignFile struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType,omitempty"`
}

// VideoPresignRequest asks for a batch of direct-to-R2 PUT URLs for an HLS
// bundle (playlist + init + segments), mirroring the convert/ad presign flow so
// the browser never streams video bytes through the container.
type VideoPresignRequest struct {
	// Files are every part of the bundle (playlist(s), init, segments). Required.
	Files []VideoPresignFile `json:"files"`
	// Prefix is an optional target R2 key prefix; defaults to hls/{uuid}/.
	Prefix string `json:"prefix,omitempty"`
}

// HLSContentType maps an HLS bundle filename to its MIME type, defaulting to
// application/octet-stream for unknown extensions. Shared by the upload/presign
// path and the stream proxy so stored and served content types agree.
func HLSContentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	case ".m4s", ".mp4":
		return "video/mp4"
	case ".vtt":
		return "text/vtt"
	case ".aac":
		return "audio/aac"
	default:
		return "application/octet-stream"
	}
}

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
