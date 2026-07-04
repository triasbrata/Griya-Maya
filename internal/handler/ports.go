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
	Recommendations(ctx context.Context, sourceID string, genres, exclude []string, page int) (domain.MediaPage, error)
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

// SourceService is the source listing + management port the SourceHandler
// depends on (implemented by *service.SourceService).
type SourceService interface {
	List(ctx context.Context, enabledOnly bool) ([]domain.Source, error)
	Get(ctx context.Context, id string) (domain.Source, error)
	Create(ctx context.Context, req domain.SourceWriteRequest) (domain.Source, error)
	Update(ctx context.Context, id string, req domain.SourceWriteRequest) (domain.Source, error)
	Delete(ctx context.Context, id string) error
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

// ConnectionService is the external-source OAuth connection port the
// ConnectionHandler depends on (implemented by *service.ConnectionService).
type ConnectionService interface {
	Create(ctx context.Context, req domain.ConnectionWriteRequest) (domain.Connection, error)
	List(ctx context.Context) ([]domain.Connection, error)
	Get(ctx context.Context, id string) (domain.Connection, error)
	Update(ctx context.Context, id string, req domain.ConnectionWriteRequest) (domain.Connection, error)
	Delete(ctx context.Context, id string) error
	Authorize(ctx context.Context, id, redirectURI string) (string, error)
	Callback(ctx context.Context, code, state string) (domain.Connection, error)
	Refresh(ctx context.Context, id string) (domain.Connection, error)
	Search(ctx context.Context, id, query, kind string, limit int) ([]domain.MediaSuggestion, error)
}
