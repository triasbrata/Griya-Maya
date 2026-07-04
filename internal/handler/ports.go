package handler

import (
	"context"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
)

// MediaService is the catalog/reader + media/chapter management port the
// MediaHandler depends on (implemented by *service.MediaService). Depending on
// the interface keeps the HTTP layer testable with a generated mock.
type MediaService interface {
	// Reads.
	Popular(ctx context.Context, sourceID string, page int, filter domain.CatalogFilter) (domain.MediaPage, error)
	Latest(ctx context.Context, sourceID string, page int, filter domain.CatalogFilter) (domain.MediaPage, error)
	Search(ctx context.Context, sourceID, query string, page int, filter domain.CatalogFilter) (domain.MediaPage, error)
	Genres(ctx context.Context, sourceID string) ([]domain.Taxonomy, error)
	Categories(ctx context.Context, sourceID string) ([]domain.Taxonomy, error)
	Details(ctx context.Context, id string) (domain.Media, error)
	Chapters(ctx context.Context, mediaID string) ([]domain.Chapter, error)
	Pages(ctx context.Context, chapterID string) ([]domain.Page, error)

	// Media + chapter management.
	CreateMedia(ctx context.Context, req domain.MediaWriteRequest) (domain.Media, error)
	UpdateMedia(ctx context.Context, id string, req domain.MediaWriteRequest) (domain.Media, error)
	DeleteMedia(ctx context.Context, id string) error
	CreateChapter(ctx context.Context, req domain.ChapterWriteRequest) (domain.Chapter, error)
	UpdateChapter(ctx context.Context, id string, req domain.ChapterWriteRequest) (domain.Chapter, error)
	DeleteChapter(ctx context.Context, id string) error
}

// TaxonomyService is the taxonomy-management port the TaxonomyHandler depends on
// (implemented by *service.TaxonomyService).
type TaxonomyService interface {
	List(ctx context.Context, kind domain.TaxonomyKind) ([]domain.Taxonomy, error)
	Create(ctx context.Context, kind domain.TaxonomyKind, name string) (domain.Taxonomy, error)
	Update(ctx context.Context, kind domain.TaxonomyKind, id, name string) (domain.Taxonomy, error)
	Delete(ctx context.Context, kind domain.TaxonomyKind, id string) error
}

// ConvertService is the conversion port the ConvertHandler depends on
// (implemented by *service.ConvertService).
type ConvertService interface {
	Convert(ctx context.Context, req domain.ConvertRequest) (service.ConvertResult, error)
	Job(ctx context.Context, id string) (domain.ConvertJob, error)
}

// VideoService is the HLS registration port the VideoHandler depends on
// (implemented by *service.VideoService).
type VideoService interface {
	Register(ctx context.Context, req domain.VideoRegisterRequest) (domain.Page, error)
}

// NovelService is the text-chapter registration port the NovelHandler depends on
// (implemented by *service.NovelService).
type NovelService interface {
	Register(ctx context.Context, req domain.NovelRegisterRequest) (domain.Page, error)
}
