package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// MediaService serves the catalog/reader endpoints and the media/chapter
// management endpoints for the unified media entity (manga | video | novel).
type MediaService struct {
	repo          MediaRepository
	store         ObjectStore
	publicBaseURL string
	presignTTL    time.Duration
}

// NewMediaService wires a MediaService. presignTTL bounds how long the direct
// R2 page URLs it mints stay valid.
func NewMediaService(repo MediaRepository, store ObjectStore, publicBaseURL string, presignTTL time.Duration) *MediaService {
	return &MediaService{
		repo:          repo,
		store:         store,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		presignTTL:    presignTTL,
	}
}

// Popular returns the most popular media for a source, honoring the filter.
func (s *MediaService) Popular(ctx context.Context, sourceID string, page int, filter domain.CatalogFilter) (domain.MediaPage, error) {
	return s.repo.List(ctx, sourceID, "popular", page, domain.CatalogPageSize, filter)
}

// Latest returns the most recently updated media for a source, honoring the filter.
func (s *MediaService) Latest(ctx context.Context, sourceID string, page int, filter domain.CatalogFilter) (domain.MediaPage, error) {
	return s.repo.List(ctx, sourceID, "latest", page, domain.CatalogPageSize, filter)
}

// Search matches titles within a source, honoring the filter.
func (s *MediaService) Search(ctx context.Context, sourceID, query string, page int, filter domain.CatalogFilter) (domain.MediaPage, error) {
	return s.repo.Search(ctx, sourceID, query, page, domain.CatalogPageSize, filter)
}

// Recommendations returns content-based recommendations for a source: its
// catalog ranked by genre overlap with genres (which the client aggregates from
// the user's recent reading — history stays on the client). Media whose id is in
// exclude, or that share no requested genre, are omitted. With no genres it falls
// back to the source's popular feed so the endpoint always returns something.
func (s *MediaService) Recommendations(ctx context.Context, sourceID string, genres, exclude []string, page int) (domain.MediaPage, error) {
	if len(genres) == 0 {
		return s.repo.List(ctx, sourceID, "popular", page, domain.CatalogPageSize, domain.CatalogFilter{})
	}
	return s.repo.Recommend(ctx, sourceID, genres, exclude, page, domain.CatalogPageSize)
}

// Genres lists the distinct filterable genres seen across a source's catalog.
func (s *MediaService) Genres(ctx context.Context, sourceID string) ([]domain.Taxonomy, error) {
	return s.repo.Genres(ctx, sourceID)
}

// Categories lists the distinct filterable categories seen across a source's catalog.
func (s *MediaService) Categories(ctx context.Context, sourceID string) ([]domain.Taxonomy, error) {
	return s.repo.Categories(ctx, sourceID)
}

// Details returns one media entry.
func (s *MediaService) Details(ctx context.Context, id string) (domain.Media, error) {
	return s.repo.Get(ctx, id)
}

// Chapters returns a media entry's chapters.
func (s *MediaService) Chapters(ctx context.Context, mediaID string) ([]domain.Chapter, error) {
	return s.repo.Chapters(ctx, mediaID)
}

// ChapterNeighbors returns the chapters immediately before and after the given
// chapter within its media, ordered by chapter number. Either side is nil at the
// ends of the list. The chapter must exist (else domain.ErrNotFound).
func (s *MediaService) ChapterNeighbors(ctx context.Context, chapterID string) (domain.ChapterNeighbors, error) {
	current, err := s.repo.ChapterByID(ctx, chapterID)
	if err != nil {
		return domain.ChapterNeighbors{}, err
	}
	siblings, err := s.repo.Chapters(ctx, current.MediaID)
	if err != nil {
		return domain.ChapterNeighbors{}, err
	}
	var out domain.ChapterNeighbors
	for i, ch := range siblings {
		if ch.ID != current.ID {
			continue
		}
		if i > 0 {
			prev := siblings[i-1]
			out.Previous = &prev
		}
		if i < len(siblings)-1 {
			next := siblings[i+1]
			out.Next = &next
		}
		break
	}
	return out, nil
}

// Pages returns a chapter's readable pages with fetchable URLs.
func (s *MediaService) Pages(ctx context.Context, chapterID string) ([]domain.Page, error) {
	stored, err := s.repo.Pages(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	pages := make([]domain.Page, 0, len(stored))
	for _, sp := range stored {
		page := domain.Page{
			Index:  sp.Index,
			Width:  sp.Width,
			Height: sp.Height,
		}
		switch sp.Kind {
		case domain.PageKindVideo:
			// HLS: the client streams the `.m3u8`. Delivery must be path-based so
			// relative segment URIs inside the playlist resolve.
			page.Type = domain.PageKindVideo
			page.ImageURL = streamURL(s.store, s.publicBaseURL, sp.R2Key)
		case domain.PageKindNovel:
			// Novel: inline the `.txt` from R2 so the client's novel reader renders
			// Body directly. ImageURL stays empty.
			text, err := s.novelBody(ctx, sp.R2Key)
			if err != nil {
				return nil, err
			}
			page.Type = domain.PageKindNovel
			page.Body = text
		default:
			u, err := s.pageURL(ctx, sp.R2Key)
			if err != nil {
				return nil, err
			}
			page.ImageURL = u
		}
		pages = append(pages, page)
	}
	return pages, nil
}

// --- media management ---

// CreateMedia validates and persists a new media entry, returning the stored
// (normalized) row.
func (s *MediaService) CreateMedia(ctx context.Context, req domain.MediaWriteRequest) (domain.Media, error) {
	m, err := mediaFromRequest(uuid.NewString(), req)
	if err != nil {
		return domain.Media{}, err
	}
	if err := s.repo.CreateMedia(ctx, m); err != nil {
		return domain.Media{}, fmt.Errorf("create media: %w", err)
	}
	return s.repo.Get(ctx, m.ID)
}

// UpdateMedia rewrites an existing media entry, returning the stored row.
func (s *MediaService) UpdateMedia(ctx context.Context, id string, req domain.MediaWriteRequest) (domain.Media, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Media{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	m, err := mediaFromRequest(id, req)
	if err != nil {
		return domain.Media{}, err
	}
	if err := s.repo.UpdateMedia(ctx, m); err != nil {
		return domain.Media{}, fmt.Errorf("update media: %w", err)
	}
	return s.repo.Get(ctx, id)
}

// DeleteMedia removes a media entry and its chapters/pages/links.
func (s *MediaService) DeleteMedia(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.repo.DeleteMedia(ctx, id)
}

// mediaFromRequest validates a write request and builds a domain.Media (id set
// by the caller on create, passed through on update).
func mediaFromRequest(id string, req domain.MediaWriteRequest) (domain.Media, error) {
	if strings.TrimSpace(req.SourceID) == "" {
		return domain.Media{}, fmt.Errorf("%w: sourceId is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(req.Title) == "" {
		return domain.Media{}, fmt.Errorf("%w: title is required", domain.ErrInvalidInput)
	}
	// URL is the entry's identifier in the Mihon source contract. This is a
	// self-hosted catalog (the "source" is us), so operators don't invent a
	// source URL — default it to the id, which the reader can address directly.
	url := strings.TrimSpace(req.URL)
	if url == "" {
		url = id
	}
	mtype := req.Type
	if mtype == "" {
		mtype = domain.MediaManga
	}
	switch mtype {
	case domain.MediaManga, domain.MediaVideo, domain.MediaNovel:
	default:
		return domain.Media{}, fmt.Errorf("%w: type must be manga, video, or novel", domain.ErrInvalidInput)
	}
	status := req.Status
	if status == "" {
		status = domain.StatusUnknown
	}
	return domain.Media{
		ID:          id,
		SourceID:    req.SourceID,
		Type:        mtype,
		URL:         url,
		Title:       req.Title,
		CoverURL:    req.CoverURL,
		Description: req.Description,
		Status:      status,
		Genres:      req.Genres,
		Categories:  req.Categories,
		Authors:     req.Authors,
		Artists:     req.Artists,
	}, nil
}

// --- chapter management ---

// CreateChapter validates and persists a new chapter for a media entry.
func (s *MediaService) CreateChapter(ctx context.Context, req domain.ChapterWriteRequest) (domain.Chapter, error) {
	c, err := chapterFromRequest("", req)
	if err != nil {
		return domain.Chapter{}, err
	}
	c.ID = uuid.NewString()
	if err := s.repo.CreateChapter(ctx, c); err != nil {
		return domain.Chapter{}, fmt.Errorf("create chapter: %w", err)
	}
	return c, nil
}

// UpdateChapter rewrites an existing chapter's mutable fields.
func (s *MediaService) UpdateChapter(ctx context.Context, id string, req domain.ChapterWriteRequest) (domain.Chapter, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Chapter{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	c, err := chapterFromRequest(id, req)
	if err != nil {
		return domain.Chapter{}, err
	}
	if err := s.repo.UpdateChapter(ctx, c); err != nil {
		return domain.Chapter{}, fmt.Errorf("update chapter: %w", err)
	}
	return c, nil
}

// DeleteChapter removes a chapter and its pages.
func (s *MediaService) DeleteChapter(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.repo.DeleteChapter(ctx, id)
}

func chapterFromRequest(id string, req domain.ChapterWriteRequest) (domain.Chapter, error) {
	if strings.TrimSpace(req.MediaID) == "" {
		return domain.Chapter{}, fmt.Errorf("%w: mediaId is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(req.Name) == "" {
		return domain.Chapter{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	return domain.Chapter{
		ID:         id,
		MediaID:    req.MediaID,
		URL:        req.URL,
		Name:       req.Name,
		Number:     req.Number,
		Scanlator:  req.Scanlator,
		DateUpload: req.DateUpload,
		Format:     req.Format,
	}, nil
}

// --- URL builders ---

// pageURL prefers a public/custom R2 domain; otherwise it mints a short-lived
// SigV4 presigned GET URL so the client fetches AVIF bytes straight from the
// private R2 bucket (no container proxy hop). The signature self-expires after
// presignTTL, so access is gated by *needing a manga.read token to mint it*.
func (s *MediaService) pageURL(ctx context.Context, key string) (string, error) {
	if u := s.store.PublicURL(key); u != "" {
		return u, nil
	}
	return s.store.PresignGet(ctx, key, s.presignTTL)
}

// novelBody fetches a novel chapter's `.txt` object from R2 and returns its text
// so the pages response can inline it as Page.Body.
func (s *MediaService) novelBody(ctx context.Context, key string) (string, error) {
	data, _, err := s.store.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// streamURL builds the fetchable URL for an HLS object (playlist or segment).
// It prefers a public/custom R2 domain; otherwise it routes through this
// service's path-based `/v1/stream` proxy so a playlist's relative segment URIs
// resolve against the same directory. Shared by MediaService (page URLs) and
// VideoService (registration response).
func streamURL(store ObjectStore, publicBaseURL, key string) string {
	if u := store.PublicURL(key); u != "" {
		return u
	}
	return publicBaseURL + "/v1/stream/" + strings.TrimLeft(key, "/")
}
