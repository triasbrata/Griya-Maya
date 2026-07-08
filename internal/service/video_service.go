package service

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// videoPresignPutTTL bounds how long a minted direct-upload URL stays valid.
const videoPresignPutTTL = 30 * time.Minute

// maxVideoPresignBatch caps how many upload URLs a single presign request mints
// (an HLS ladder is many segments, but this backstops abuse).
const maxVideoPresignBatch = 5000

// VideoPresignItem is one bundle file's target R2 key, the presigned PUT URL,
// and the Content-Type the client MUST send (it is bound into the signature).
type VideoPresignItem struct {
	Name        string `json:"name"`
	Key         string `json:"key"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
}

// VideoPresignResult is the batch of upload targets under one fresh prefix plus
// the detected playlist key to hand back to Register.
type VideoPresignResult struct {
	Prefix      string             `json:"prefix"`
	PlaylistKey string             `json:"playlistKey"`
	Items       []VideoPresignItem `json:"items"`
}

// PresignUploads mints a presigned R2 PUT URL for every file in an HLS bundle so
// the browser uploads segments straight to R2 (no container hop). Keys keep the
// original basenames under one shared prefix so a playlist's relative segment
// URIs resolve. The playlist key (prefer index.m3u8) is returned for Register.
func (s *VideoService) PresignUploads(ctx context.Context, req domain.VideoPresignRequest) (VideoPresignResult, error) {
	if len(req.Files) == 0 || len(req.Files) > maxVideoPresignBatch {
		return VideoPresignResult{}, fmt.Errorf("%w: files must be between 1 and %d", domain.ErrInvalidInput, maxVideoPresignBatch)
	}

	prefix := strings.TrimLeft(strings.TrimSpace(req.Prefix), "/")
	if prefix == "" {
		prefix = "hls/" + uuid.NewString() + "/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	items := make([]VideoPresignItem, 0, len(req.Files))
	playlistKey := ""
	for _, f := range req.Files {
		name := path.Base(strings.TrimSpace(f.Name))
		if name == "" || name == "." || name == "/" {
			return VideoPresignResult{}, fmt.Errorf("%w: a file has an empty name", domain.ErrInvalidInput)
		}
		// Bind the extension-derived type into the signature so stored objects
		// match what the stream proxy serves; a client-sent type wins only when
		// provided (it must then PUT with that exact header).
		ct := strings.TrimSpace(f.ContentType)
		if ct == "" {
			ct = domain.HLSContentType(name)
		}
		key := prefix + name
		url, err := s.store.PresignPut(ctx, key, videoPresignPutTTL, ct)
		if err != nil {
			return VideoPresignResult{}, fmt.Errorf("presign %q: %w", name, err)
		}
		items = append(items, VideoPresignItem{Name: name, Key: key, URL: url, ContentType: ct})

		if strings.EqualFold(path.Ext(name), ".m3u8") {
			// Prefer a master/index playlist; otherwise keep the first .m3u8 seen.
			if playlistKey == "" || strings.EqualFold(name, "index.m3u8") {
				playlistKey = key
			}
		}
	}
	if playlistKey == "" {
		return VideoPresignResult{}, fmt.Errorf("%w: no .m3u8 playlist among the files", domain.ErrInvalidInput)
	}

	return VideoPresignResult{Prefix: prefix, PlaylistKey: playlistKey, Items: items}, nil
}

// VideoService registers an already-uploaded HLS bundle (in R2) as a chapter's
// single video page. Segmenting/transcoding is out of scope: the bundle is
// produced elsewhere and uploaded via the video upload endpoint; this service
// only wires the playlist key to a chapter and hands back a playable URL.
type VideoService struct {
	pages         JobRepository
	store         ObjectStore
	publicBaseURL string
}

// NewVideoService wires a VideoService. It reuses JobRepository.ReplacePages to
// persist the (single) video page for a chapter.
func NewVideoService(pages JobRepository, store ObjectStore, publicBaseURL string) *VideoService {
	return &VideoService{
		pages:         pages,
		store:         store,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

// Register associates an HLS playlist already in R2 with a chapter, rewriting
// the chapter's pages to a single video page. Returns the resulting page (with
// its resolved streaming URL) for confirmation.
func (s *VideoService) Register(ctx context.Context, req domain.VideoRegisterRequest) (domain.Page, error) {
	chapterID := strings.TrimSpace(req.ChapterID)
	if chapterID == "" {
		return domain.Page{}, fmt.Errorf("%w: chapterId is required", domain.ErrInvalidInput)
	}
	playlistKey := strings.TrimLeft(strings.TrimSpace(req.PlaylistKey), "/")
	if playlistKey == "" {
		return domain.Page{}, fmt.Errorf("%w: playlistKey is required", domain.ErrInvalidInput)
	}
	if !strings.HasSuffix(strings.ToLower(playlistKey), ".m3u8") {
		return domain.Page{}, fmt.Errorf("%w: playlistKey must point at an .m3u8 playlist", domain.ErrInvalidInput)
	}

	stored := []domain.StoredPage{{
		Index:  0,
		R2Key:  playlistKey,
		Width:  req.Width,
		Height: req.Height,
		Kind:   domain.PageKindVideo,
	}}
	if err := s.pages.ReplacePages(ctx, chapterID, stored); err != nil {
		return domain.Page{}, fmt.Errorf("persist video page: %w", err)
	}

	return domain.Page{
		Index:    0,
		Type:     domain.PageKindVideo,
		ImageURL: streamURL(s.store, s.publicBaseURL, playlistKey),
		Width:    req.Width,
		Height:   req.Height,
	}, nil
}
