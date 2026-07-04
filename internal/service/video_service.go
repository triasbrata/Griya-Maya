package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

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
