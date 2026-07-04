// Package service holds the business logic layer: it orchestrates repositories
// (D1/R2) and the convert engine behind small interfaces (ports), so handlers
// depend on behavior rather than concrete storage.
package service

import (
	"context"

	"github.com/triasbrata/mihon-manga-server/internal/convert"
	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// MangaRepository is the catalog persistence port (implemented by d1.MangaRepo).
type MangaRepository interface {
	List(ctx context.Context, sourceID, order string, page, perPage int, filter domain.CatalogFilter) (domain.MangaPage, error)
	Search(ctx context.Context, sourceID, query string, page, perPage int, filter domain.CatalogFilter) (domain.MangaPage, error)
	Genres(ctx context.Context, sourceID string) ([]domain.GenreTag, error)
	Get(ctx context.Context, id string) (domain.Manga, error)
	Chapters(ctx context.Context, mangaID string) ([]domain.Chapter, error)
	Pages(ctx context.Context, chapterID string) ([]domain.StoredPage, error)
}

// JobRepository persists conversion jobs (implemented by d1.JobRepo).
type JobRepository interface {
	Create(ctx context.Context, job domain.ConvertJob) error
	UpdateStatus(ctx context.Context, id string, status domain.ConvertStatus, pageCount int, errMsg string) error
	Get(ctx context.Context, id string) (domain.ConvertJob, error)
	ReplacePages(ctx context.Context, chapterID string, pages []domain.StoredPage) error
}

// ObjectStore is the blob port (implemented by r2.Store).
type ObjectStore interface {
	Get(ctx context.Context, key string) ([]byte, string, error)
	Put(ctx context.Context, key string, data []byte, contentType string) error
	PublicURL(key string) string
}

// ArchiveConverter turns an archive into AVIF pages (implemented by convert.Converter).
type ArchiveConverter interface {
	Convert(ctx context.Context, format domain.ArchiveFormat, archive []byte) ([]convert.Result, error)
}
