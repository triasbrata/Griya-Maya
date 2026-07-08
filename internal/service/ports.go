// Package service holds the business logic layer: it orchestrates repositories
// (D1/R2) and the convert engine behind small interfaces (ports), so handlers
// depend on behavior rather than concrete storage.
package service

import (
	"context"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// MediaRepository is the catalog persistence port (implemented by d1.MediaRepo).
// It covers reads (browse/detail/pages), media + chapter writes, and taxonomy
// management for the unified media entity.
type MediaRepository interface {
	// Reads.
	List(ctx context.Context, sourceID, order string, page, perPage int, filter domain.CatalogFilter) (domain.MediaPage, error)
	Search(ctx context.Context, sourceID, query string, page, perPage int, filter domain.CatalogFilter) (domain.MediaPage, error)
	// Recommend ranks a source's media by shared sub-type with subTypes (desc),
	// tie-broken by the popular order; non-matching and exclude ids are omitted.
	Recommend(ctx context.Context, sourceID string, subTypes, exclude []string, page, perPage int) (domain.MediaPage, error)
	// SubTypes lists the distinct sub-types present in a source's catalog.
	SubTypes(ctx context.Context, sourceID string) ([]domain.SubType, error)
	Get(ctx context.Context, id string) (domain.Media, error)
	Chapters(ctx context.Context, mediaID string) ([]domain.Chapter, error)
	ChapterByID(ctx context.Context, id string) (domain.Chapter, error)
	Pages(ctx context.Context, chapterID string) ([]domain.StoredPage, error)

	// Media writes.
	CreateMedia(ctx context.Context, m domain.Media) error
	UpdateMedia(ctx context.Context, m domain.Media) error
	// SetMediaCover rewrites only cover_url (used by the async cover mirror to
	// swap an external URL for the stored R2 key).
	SetMediaCover(ctx context.Context, mediaID, coverURL string) error
	DeleteMedia(ctx context.Context, id string) error

	// Chapter writes.
	CreateChapter(ctx context.Context, c domain.Chapter) error
	UpdateChapter(ctx context.Context, c domain.Chapter) error
	DeleteChapter(ctx context.Context, id string) error
	// DeletePage removes a single page row (chapter_id + idx). The caller reads
	// the page's R2 key first so it can schedule the artifact for cleanup.
	DeletePage(ctx context.Context, chapterID string, idx int) error
	// PageKeysForMedia returns every stored R2 key across all of a media entry's
	// chapters, used to schedule R2 cleanup on media deletion.
	PageKeysForMedia(ctx context.Context, mediaID string) ([]string, error)

	// Taxonomy management (genre/author/artist).
	ListTaxonomy(ctx context.Context, kind domain.TaxonomyKind) ([]domain.Taxonomy, error)
	CreateTaxonomy(ctx context.Context, kind domain.TaxonomyKind, name string) (domain.Taxonomy, error)
	UpdateTaxonomy(ctx context.Context, kind domain.TaxonomyKind, id, name string) (domain.Taxonomy, error)
	DeleteTaxonomy(ctx context.Context, kind domain.TaxonomyKind, id string) error

	// Managed sub-type vocabulary (per-type `sub_type` table).
	// SubTypeVocab returns the full vocabulary grouped by media type.
	SubTypeVocab(ctx context.Context) (map[domain.MediaType][]domain.SubType, error)
	// ValidSubType reports whether slug is allowed for media type t (empty is ok).
	ValidSubType(ctx context.Context, t domain.MediaType, slug string) (bool, error)
	CreateSubType(ctx context.Context, st domain.SubType) error
	UpdateSubType(ctx context.Context, slug string, st domain.SubType) error
	DeleteSubType(ctx context.Context, slug string) error
}

// JobRepository persists a chapter's page rows (implemented by d1.JobRepo). It
// is the shared write path used by the presign/register ingest flow and by the
// video/novel registration services.
type JobRepository interface {
	ReplacePages(ctx context.Context, chapterID string, pages []domain.StoredPage) error
}

// CoverMirrorQueue enqueues external cover images for async mirroring into R2
// (implemented by a Cloudflare Queue producer; a no-op when the queue is off).
type CoverMirrorQueue interface {
	Enqueue(ctx context.Context, job domain.CoverMirrorJob) error
}

// CleanupQueue enqueues orphaned R2 object keys for async deletion after their
// D1 rows have been removed (implemented by a Cloudflare Queue producer; a
// no-op when the queue is off, so deletions still succeed with the artifacts
// left behind rather than failing the request).
type CleanupQueue interface {
	Enqueue(ctx context.Context, keys []string) error
	// EnqueuePrefixes schedules recursive deletion of every object under each
	// prefix (HLS video bundles, where only the playlist key is recorded).
	EnqueuePrefixes(ctx context.Context, prefixes []string) error
}

// SourceRepository persists content sources (implemented by d1.SourceRepo).
type SourceRepository interface {
	List(ctx context.Context, enabledOnly bool) ([]domain.Source, error)
	Get(ctx context.Context, id string) (domain.Source, error)
	Exists(ctx context.Context, id string) (bool, error)
	MediaCount(ctx context.Context, id string) (int, error)
	Create(ctx context.Context, s domain.Source) error
	Update(ctx context.Context, s domain.Source) error
	Delete(ctx context.Context, id string) error
}

// AdRepository persists house-ad creatives (implemented by d1.AdRepo).
type AdRepository interface {
	// List returns ads ordered by weight (desc). activeOnly restricts to active
	// ads (the reader listing); a non-empty placement filters to that placement.
	List(ctx context.Context, activeOnly bool, placement string) ([]domain.StoredAd, error)
	Get(ctx context.Context, id string) (domain.StoredAd, error)
	Create(ctx context.Context, a domain.StoredAd) error
	Update(ctx context.Context, a domain.StoredAd) error
	Delete(ctx context.Context, id string) error
}

// ObjectStore is the blob port (implemented by r2.Store).
type ObjectStore interface {
	Get(ctx context.Context, key string) ([]byte, string, error)
	Put(ctx context.Context, key string, data []byte, contentType string) error
	PublicURL(key string) string
	// PresignGet mints a short-lived direct-fetch URL for key so the client
	// pulls bytes from R2 without a proxy hop.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	// PresignPut mints a short-lived direct-upload URL for key so the client
	// pushes bytes to R2 without a proxy hop. contentType, when non-empty, binds
	// the required Content-Type header into the signature.
	PresignPut(ctx context.Context, key string, ttl time.Duration, contentType string) (string, error)
	// DeleteObjects removes objects by key (batched; "not found" is success).
	DeleteObjects(ctx context.Context, keys []string) error
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
