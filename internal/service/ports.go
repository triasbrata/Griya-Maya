// Package service holds the business logic layer: it orchestrates repositories
// (D1/R2) and the convert engine behind small interfaces (ports), so handlers
// depend on behavior rather than concrete storage.
package service

import (
	"context"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/convert"
	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// MediaRepository is the catalog persistence port (implemented by d1.MediaRepo).
// It covers reads (browse/detail/pages), media + chapter writes, and taxonomy
// management for the unified media entity.
type MediaRepository interface {
	// Reads.
	List(ctx context.Context, sourceID, order string, page, perPage int, filter domain.CatalogFilter) (domain.MediaPage, error)
	Search(ctx context.Context, sourceID, query string, page, perPage int, filter domain.CatalogFilter) (domain.MediaPage, error)
	// Recommend ranks a source's media by genre overlap with genres (desc), tie-
	// broken by the popular order; zero-overlap and exclude ids are omitted.
	Recommend(ctx context.Context, sourceID string, genres, exclude []string, page, perPage int) (domain.MediaPage, error)
	Genres(ctx context.Context, sourceID string) ([]domain.Taxonomy, error)
	Categories(ctx context.Context, sourceID string) ([]domain.Taxonomy, error)
	Get(ctx context.Context, id string) (domain.Media, error)
	Chapters(ctx context.Context, mediaID string) ([]domain.Chapter, error)
	Pages(ctx context.Context, chapterID string) ([]domain.StoredPage, error)

	// Media writes.
	CreateMedia(ctx context.Context, m domain.Media) error
	UpdateMedia(ctx context.Context, m domain.Media) error
	DeleteMedia(ctx context.Context, id string) error

	// Chapter writes.
	CreateChapter(ctx context.Context, c domain.Chapter) error
	UpdateChapter(ctx context.Context, c domain.Chapter) error
	DeleteChapter(ctx context.Context, id string) error

	// Taxonomy management (genre/category/author/artist).
	ListTaxonomy(ctx context.Context, kind domain.TaxonomyKind) ([]domain.Taxonomy, error)
	CreateTaxonomy(ctx context.Context, kind domain.TaxonomyKind, name string) (domain.Taxonomy, error)
	UpdateTaxonomy(ctx context.Context, kind domain.TaxonomyKind, id, name string) (domain.Taxonomy, error)
	DeleteTaxonomy(ctx context.Context, kind domain.TaxonomyKind, id string) error
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
	// PresignGet mints a short-lived direct-fetch URL for key so the client
	// pulls bytes from R2 without a proxy hop.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// ArchiveConverter turns an archive into AVIF pages (implemented by convert.Converter).
type ArchiveConverter interface {
	Convert(ctx context.Context, format domain.ArchiveFormat, archive []byte) ([]convert.Result, error)
}

// ConnectionRepository persists external-source OAuth connections (implemented
// by d1.ConnectionRepo). Secret/token fields are stored as opaque ciphertext.
type ConnectionRepository interface {
	Create(ctx context.Context, c domain.Connection) error
	List(ctx context.Context) ([]domain.Connection, error)
	Get(ctx context.Context, id string) (domain.Connection, error)
	Update(ctx context.Context, c domain.Connection) error
	Delete(ctx context.Context, id string) error
	SaveTokens(ctx context.Context, id, access, refresh, tokenType string, expiresAt int64, status domain.ConnectionStatus, updatedAt int64) error
}

// OAuthClient performs the outbound OAuth2 token exchange/refresh against an
// external provider (implemented by oauth.Client).
type OAuthClient interface {
	Exchange(ctx context.Context, p domain.Provider, clientID, clientSecret, code, codeVerifier, redirectURI string) (domain.TokenResponse, error)
	Refresh(ctx context.Context, p domain.Provider, clientID, clientSecret, refreshToken string) (domain.TokenResponse, error)
	// Get performs an authenticated GET against url with accessToken as the
	// Bearer credential, returning the raw body and HTTP status (non-2xx is not
	// an error) so the caller can refresh-on-401 and retry.
	Get(ctx context.Context, url, accessToken string) ([]byte, int, error)
}

// StateStore persists the short-lived PKCE/state bundle between the authorize
// redirect and the callback (implemented by kv.StateStore). Get is single-use.
type StateStore interface {
	Put(ctx context.Context, state string, v domain.AuthState, ttlSeconds int) error
	Get(ctx context.Context, state string) (domain.AuthState, error)
}
