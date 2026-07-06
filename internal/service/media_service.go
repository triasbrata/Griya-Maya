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
	coverQueue    CoverMirrorQueue
	cleanupQueue  CleanupQueue
	publicBaseURL string
	presignTTL    time.Duration
}

// NewMediaService wires a MediaService. presignTTL bounds how long the direct
// R2 page URLs it mints stay valid. coverQueue enqueues external cover images
// for async mirroring into R2; cleanupQueue enqueues orphaned R2 keys for async
// deletion on chapter/page/media removal (both no-op producers when their queue
// is unconfigured).
func NewMediaService(repo MediaRepository, store ObjectStore, coverQueue CoverMirrorQueue, cleanupQueue CleanupQueue, publicBaseURL string, presignTTL time.Duration) *MediaService {
	return &MediaService{
		repo:          repo,
		store:         store,
		coverQueue:    coverQueue,
		cleanupQueue:  cleanupQueue,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		presignTTL:    presignTTL,
	}
}

// isMirroredCover reports whether a stored cover value is an R2 object key (the
// mirrored form) rather than an external URL. Keys have no URL scheme.
func isMirroredCover(cover string) bool {
	return cover != "" && !strings.Contains(cover, "://")
}

// resolveCover turns a stored cover R2 key into a fetchable (presigned) URL,
// leaving external URLs (not yet mirrored, or mirror failed) untouched.
func (s *MediaService) resolveCover(ctx context.Context, m domain.Media) domain.Media {
	if isMirroredCover(m.CoverURL) {
		if u, err := s.pageURL(ctx, m.CoverURL); err == nil {
			m.CoverURL = u
		}
	}
	return m
}

// resolveCovers applies resolveCover across a page of media.
func (s *MediaService) resolveCovers(ctx context.Context, p domain.MediaPage) domain.MediaPage {
	for i := range p.Items {
		p.Items[i] = s.resolveCover(ctx, p.Items[i])
	}
	return p
}

// enqueueCoverMirror schedules an external cover image to be mirrored into R2 as
// AVIF (best-effort: a failed enqueue never fails the write).
func (s *MediaService) enqueueCoverMirror(ctx context.Context, mediaID, cover string) {
	if s.coverQueue == nil || cover == "" || isMirroredCover(cover) {
		return
	}
	_ = s.coverQueue.Enqueue(ctx, domain.CoverMirrorJob{MediaID: mediaID, SourceURL: cover})
}

// enqueueCleanup schedules orphaned R2 object keys for async deletion after
// their rows have been removed (best-effort: a failed enqueue never fails the
// delete — the keys simply stay in R2 until a future cleanup).
func (s *MediaService) enqueueCleanup(ctx context.Context, keys []string) {
	if s.cleanupQueue == nil || len(keys) == 0 {
		return
	}
	_ = s.cleanupQueue.Enqueue(ctx, keys)
}

// Popular returns the most popular media for a source, honoring the filter.
func (s *MediaService) Popular(ctx context.Context, sourceID string, page int, filter domain.CatalogFilter) (domain.MediaPage, error) {
	res, err := s.repo.List(ctx, sourceID, "popular", page, domain.CatalogPageSize, filter)
	if err != nil {
		return domain.MediaPage{}, err
	}
	return s.resolveCovers(ctx, res), nil
}

// Latest returns the most recently updated media for a source, honoring the filter.
func (s *MediaService) Latest(ctx context.Context, sourceID string, page int, filter domain.CatalogFilter) (domain.MediaPage, error) {
	res, err := s.repo.List(ctx, sourceID, "latest", page, domain.CatalogPageSize, filter)
	if err != nil {
		return domain.MediaPage{}, err
	}
	return s.resolveCovers(ctx, res), nil
}

// Search matches titles within a source, honoring the filter.
func (s *MediaService) Search(ctx context.Context, sourceID, query string, page int, filter domain.CatalogFilter) (domain.MediaPage, error) {
	res, err := s.repo.Search(ctx, sourceID, query, page, domain.CatalogPageSize, filter)
	if err != nil {
		return domain.MediaPage{}, err
	}
	return s.resolveCovers(ctx, res), nil
}

// Recommendations returns content-based recommendations for a source: its
// catalog ranked by genre overlap with genres (which the client aggregates from
// the user's recent reading — history stays on the client). Media whose id is in
// exclude, or that share no requested genre, are omitted. With no genres it falls
// back to the source's popular feed so the endpoint always returns something.
func (s *MediaService) Recommendations(ctx context.Context, sourceID string, genres, exclude []string, page int) (domain.MediaPage, error) {
	var (
		res domain.MediaPage
		err error
	)
	if len(genres) == 0 {
		res, err = s.repo.List(ctx, sourceID, "popular", page, domain.CatalogPageSize, domain.CatalogFilter{})
	} else {
		res, err = s.repo.Recommend(ctx, sourceID, genres, exclude, page, domain.CatalogPageSize)
	}
	if err != nil {
		return domain.MediaPage{}, err
	}
	return s.resolveCovers(ctx, res), nil
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
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Media{}, err
	}
	return s.resolveCover(ctx, m), nil
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
	// Mirror an external cover into R2 (AVIF) asynchronously; the stored cover
	// stays the original URL until the mirror completes.
	s.enqueueCoverMirror(ctx, m.ID, m.CoverURL)
	saved, err := s.repo.Get(ctx, m.ID)
	if err != nil {
		return domain.Media{}, err
	}
	return s.resolveCover(ctx, saved), nil
}

// UpdateMedia rewrites an existing media entry, returning the stored row.
func (s *MediaService) UpdateMedia(ctx context.Context, id string, req domain.MediaWriteRequest) (domain.Media, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Media{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Media{}, err
	}
	m, err := mediaFromRequest(id, req)
	if err != nil {
		return domain.Media{}, err
	}
	// Preserve an already-mirrored cover when the client re-submits the presigned
	// URL of the same object (the resolved form it received on read), so we don't
	// store an expiring URL or re-mirror needlessly.
	if isMirroredCover(current.CoverURL) && strings.Contains(req.CoverURL, current.CoverURL) {
		m.CoverURL = current.CoverURL
	}
	if err := s.repo.UpdateMedia(ctx, m); err != nil {
		return domain.Media{}, fmt.Errorf("update media: %w", err)
	}
	s.enqueueCoverMirror(ctx, id, m.CoverURL)
	saved, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Media{}, err
	}
	return s.resolveCover(ctx, saved), nil
}

// DeleteMedia removes a media entry and its chapters/pages/links, then schedules
// the orphaned R2 artifacts (all its pages plus a mirrored cover) for async
// cleanup. Keys are collected before the rows are deleted; the cleanup job is
// enqueued only after the delete succeeds.
func (s *MediaService) DeleteMedia(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	keys, err := s.repo.PageKeysForMedia(ctx, id)
	if err != nil {
		return err
	}
	// A mirrored cover is stored as an R2 key (external URLs are left as-is and
	// carry nothing of ours to delete). Best-effort: a missing media just yields
	// no cover key.
	if m, err := s.repo.Get(ctx, id); err == nil && isMirroredCover(m.CoverURL) {
		keys = append(keys, m.CoverURL)
	}
	if err := s.repo.DeleteMedia(ctx, id); err != nil {
		return err
	}
	s.enqueueCleanup(ctx, keys)
	return nil
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

// DeleteChapter removes a chapter and its pages, scheduling the pages' R2
// artifacts for cleanup. It delegates to DeleteChapters so single and batch
// deletes share one path.
func (s *MediaService) DeleteChapter(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.DeleteChapters(ctx, []string{id})
}

// DeleteChapters removes one or more chapters (and their pages), then schedules
// every collected R2 page key for async cleanup as a single job. For each id it
// reads the page keys, deletes the rows, and only after all rows are gone
// enqueues the cleanup — so a failed delete never schedules a live object for
// deletion. Blank ids are skipped and unknown ids are no-ops (idempotent).
func (s *MediaService) DeleteChapters(ctx context.Context, ids []string) error {
	var keys []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		pages, err := s.repo.Pages(ctx, id)
		if err != nil {
			return err
		}
		if err := s.repo.DeleteChapter(ctx, id); err != nil {
			return err
		}
		for _, p := range pages {
			if p.R2Key != "" {
				keys = append(keys, p.R2Key)
			}
		}
	}
	s.enqueueCleanup(ctx, keys)
	return nil
}

// ChapterPagesAdmin returns a chapter's pages with their raw R2 keys and a
// short-lived presigned fetch URL — the admin-only view used to inspect and
// delete individual artifacts. Unlike the reader's Pages, it exposes r2Key.
func (s *MediaService) ChapterPagesAdmin(ctx context.Context, chapterID string) ([]domain.AdminPage, error) {
	stored, err := s.repo.Pages(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.AdminPage, 0, len(stored))
	for _, sp := range stored {
		url, err := s.pageURL(ctx, sp.R2Key)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.AdminPage{
			Index:    sp.Index,
			R2Key:    sp.R2Key,
			ImageURL: url,
			Width:    sp.Width,
			Height:   sp.Height,
			Kind:     sp.Kind,
		})
	}
	return out, nil
}

// DeleteChapterPage removes a single page (by index) from a chapter and
// schedules its R2 artifact for cleanup. The page must exist (else
// domain.ErrNotFound); its key is read before the row is deleted.
func (s *MediaService) DeleteChapterPage(ctx context.Context, chapterID string, idx int) error {
	if strings.TrimSpace(chapterID) == "" {
		return fmt.Errorf("%w: chapter id is required", domain.ErrInvalidInput)
	}
	pages, err := s.repo.Pages(ctx, chapterID)
	if err != nil {
		return err
	}
	key, found := "", false
	for _, p := range pages {
		if p.Index == idx {
			key, found = p.R2Key, true
			break
		}
	}
	if !found {
		return domain.ErrNotFound
	}
	if err := s.repo.DeletePage(ctx, chapterID, idx); err != nil {
		return err
	}
	if key != "" {
		s.enqueueCleanup(ctx, []string{key})
	}
	return nil
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
