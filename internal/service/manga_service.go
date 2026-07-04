package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// MangaService serves the catalog/reader endpoints that mirror the app's
// SourceRuntime contract.
type MangaService struct {
	repo          MangaRepository
	store         ObjectStore
	publicBaseURL string
}

// NewMangaService wires a MangaService.
func NewMangaService(repo MangaRepository, store ObjectStore, publicBaseURL string) *MangaService {
	return &MangaService{
		repo:          repo,
		store:         store,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

// Popular returns the most popular manga for a source, honoring the filter.
func (s *MangaService) Popular(ctx context.Context, sourceID string, page int, filter domain.CatalogFilter) (domain.MangaPage, error) {
	return s.repo.List(ctx, sourceID, "popular", page, 30, filter)
}

// Latest returns the most recently updated manga for a source, honoring the filter.
func (s *MangaService) Latest(ctx context.Context, sourceID string, page int, filter domain.CatalogFilter) (domain.MangaPage, error) {
	return s.repo.List(ctx, sourceID, "latest", page, 30, filter)
}

// Search matches titles within a source, honoring the filter.
func (s *MangaService) Search(ctx context.Context, sourceID, query string, page int, filter domain.CatalogFilter) (domain.MangaPage, error) {
	return s.repo.Search(ctx, sourceID, query, page, 30, filter)
}

// Genres lists the distinct filterable genres seen across a source's catalog.
func (s *MangaService) Genres(ctx context.Context, sourceID string) ([]domain.GenreTag, error) {
	return s.repo.Genres(ctx, sourceID)
}

// Details returns one manga.
func (s *MangaService) Details(ctx context.Context, id string) (domain.Manga, error) {
	return s.repo.Get(ctx, id)
}

// Chapters returns a manga's chapters.
func (s *MangaService) Chapters(ctx context.Context, mangaID string) ([]domain.Chapter, error) {
	return s.repo.Chapters(ctx, mangaID)
}

// Pages returns a chapter's readable pages with fetchable AVIF URLs.
func (s *MangaService) Pages(ctx context.Context, chapterID string) ([]domain.Page, error) {
	stored, err := s.repo.Pages(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	pages := make([]domain.Page, 0, len(stored))
	for _, sp := range stored {
		pages = append(pages, domain.Page{
			Index:    sp.Index,
			ImageURL: s.pageURL(sp.R2Key),
			Width:    sp.Width,
			Height:   sp.Height,
		})
	}
	return pages, nil
}

// pageURL prefers a public/custom R2 domain; otherwise it routes the fetch back
// through this service's image proxy endpoint.
func (s *MangaService) pageURL(key string) string {
	if u := s.store.PublicURL(key); u != "" {
		return u
	}
	return s.publicBaseURL + "/v1/image?key=" + url.QueryEscape(key)
}
