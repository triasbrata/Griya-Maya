// Package domain holds the core entities exchanged over the API.
//
// The catalog is a unified `media` model: a single entity that describes manga,
// video, and novel entries, discriminated by Media.Type. The JSON shapes are
// consumed by the Mihon iOS client, which reads Type (and per-page Page.Type) to
// pick the right reader. See docs/api-migration-media.md for the client contract.
package domain

import "time"

// MediaType discriminates the kind of a catalog entry. All three kinds share the
// same Media/Chapter/Page structure; the type only selects the client reader.
type MediaType string

const (
	MediaManga MediaType = "manga"
	MediaVideo MediaType = "video"
	MediaNovel MediaType = "novel"
)

// MediaStatus mirrors the app's status enum.
type MediaStatus string

const (
	StatusUnknown    MediaStatus = "unknown"
	StatusOngoing    MediaStatus = "ongoing"
	StatusCompleted  MediaStatus = "completed"
	StatusLicensed   MediaStatus = "licensed"
	StatusHiatus     MediaStatus = "hiatus"
	StatusCancelled  MediaStatus = "cancelled"
	StatusPublishing MediaStatus = "publishing_finished"
)

// Media is a catalog entry (list/search item and detail base) for any media
// kind. Genres/Authors/Artists are normalized in storage and surfaced here as
// display-name arrays. SubType is a single classifier bound to Type (e.g.
// manga|manhwa|manhua for a manga), validated against the managed sub_type
// vocabulary.
type Media struct {
	ID          string      `json:"id"`
	SourceID    string      `json:"sourceId"`
	Type        MediaType   `json:"type"`
	SubType     string      `json:"subType,omitempty"`
	URL         string      `json:"url"`
	Title       string      `json:"title"`
	CoverURL    string      `json:"coverUrl,omitempty"`
	Description string      `json:"description,omitempty"`
	Genres      []string    `json:"genres,omitempty"`
	Authors     []string    `json:"authors,omitempty"`
	Artists     []string    `json:"artists,omitempty"`
	Status      MediaStatus `json:"status,omitempty"`
	UpdatedAt   time.Time   `json:"updatedAt,omitempty"`
}

// SubType is a single, type-scoped classifier for a media entry: a manga is one
// of manga|manhwa|manhua, a novel is web_novel|light_novel, a video is
// anime_movie|anime_series|tv_series. The vocabulary is managed in the DB
// (`sub_type` table) with admin CRUD. Slug is the canonical stored/wire value;
// Name is its display label; Type is the owning media type (omitted on the
// per-source distinct listing, where it is redundant).
type SubType struct {
	Slug string    `json:"slug"`
	Type MediaType `json:"type,omitempty"`
	Name string    `json:"name"`
}

// SubTypeWriteRequest is the admin create/update payload for a managed sub-type.
// Slug is the immutable key (required on create), Type must be one of
// manga|novel|video, and Name is the display label.
type SubTypeWriteRequest struct {
	Slug string    `json:"slug"`
	Type MediaType `json:"type"`
	Name string    `json:"name"`
}

// MediaPage is a paginated slice of catalog entries.
type MediaPage struct {
	Items   []Media `json:"items"`
	HasNext bool    `json:"hasNext"`
	Page    int     `json:"page"`
}

// Chapter is a single chapter belonging to a media entry.
type Chapter struct {
	ID         string    `json:"id"`
	MediaID    string    `json:"mediaId"`
	URL        string    `json:"url"`
	Name       string    `json:"name"`
	Number     float64   `json:"number"`
	Scanlator  string    `json:"scanlator,omitempty"`
	DateUpload time.Time `json:"dateUpload,omitempty"`
	// Format is the stored artifact type when this chapter was ingested from an
	// archive ("cbz" | "epub" | "pdf" | ""). Empty means remote-only.
	Format string `json:"format,omitempty"`
}

// Page kinds. Empty is treated as PageKindImage for backward compatibility.
const (
	// PageKindImage is a still AVIF page (the default).
	PageKindImage = "image"
	// PageKindVideo is an HLS stream: ImageURL points at an `.m3u8` playlist
	// (public R2 URL or the `/v1/stream` proxy) that the client plays with AVKit.
	PageKindVideo = "video"
	// PageKindNovel is a text chapter stored as a `.txt` object in R2. The chapter
	// is served as a single page whose Body carries the inlined text (the client's
	// novel reader renders Body, not a URL).
	PageKindNovel = "novel"
)

// Page is one readable unit. For an image page ImageURL points at an AVIF object
// in R2; for a video page (Type == PageKindVideo) it points at an HLS `.m3u8`
// playlist; for a novel page (Type == PageKindNovel) Body carries the chapter
// text and ImageURL is empty. Type is omitted for ordinary image pages so
// existing clients are unaffected.
type Page struct {
	Index    int    `json:"index"`
	ImageURL string `json:"imageUrl"`
	Type     string `json:"type,omitempty"`
	Body     string `json:"body,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// TaxonomyKind names one of the three normalized, managed taxonomies. It maps
// 1:1 to a storage table (genre/author/artist) and its join table.
type TaxonomyKind string

const (
	TaxonomyGenre  TaxonomyKind = "genre"
	TaxonomyAuthor TaxonomyKind = "author"
	TaxonomyArtist TaxonomyKind = "artist"
)

// Valid reports whether k is a known taxonomy kind.
func (k TaxonomyKind) Valid() bool {
	switch k {
	case TaxonomyGenre, TaxonomyAuthor, TaxonomyArtist:
		return true
	}
	return false
}

// HasSlug reports whether the taxonomy carries a slug (genre does; author/artist
// are name-only).
func (k TaxonomyKind) HasSlug() bool {
	return k == TaxonomyGenre
}

// Taxonomy is a normalized, managed tag: a genre, author, or artist.
// Slug is set only for genre (the app derives its filter id from it); it is
// empty (and omitted) for author/artist.
type Taxonomy struct {
	ID   string       `json:"id,omitempty"`
	Slug string       `json:"slug,omitempty"`
	Name string       `json:"name"`
	Kind TaxonomyKind `json:"-"`
}

// TaxonomyWriteRequest is the create/update payload for a taxonomy tag.
type TaxonomyWriteRequest struct {
	Name string `json:"name"`
}

// CatalogFilter carries the browse/search filter vocabulary that mirrors the
// app's `SourceFilterValue`. Zero value = unfiltered, sorted by the feed default.
type CatalogFilter struct {
	// Sort is the sort key: "popular" | "latest" | "updated" | "rating" | "title".
	// Empty falls back to the feed's natural order.
	Sort string
	// Ascending flips the sort direction (mirrors `.orderAscending`). Sorts
	// default to descending (newest/most-popular first) when false.
	Ascending bool
	// Types filters the media `type` column directly (values "manga" | "video" |
	// "novel"). Multiple types combine as OR (media.type IN (...)).
	Types []string
	// SubTypes filters the media `sub_type` column directly (e.g. "manhwa").
	// Multiple sub-types combine as OR (media.sub_type IN (...)).
	SubTypes []string
}

// SortColumn maps a filter sort key to a media column. It is the single source
// of truth for how the API's `sort` param is honored. Direction is uniform:
// descending unless the caller set Ascending (the app's `.orderAscending`).
func (f CatalogFilter) SortColumn(feedDefault string) string {
	sort := f.Sort
	if sort == "" {
		sort = feedDefault
	}
	switch sort {
	case "title":
		return "title"
	case "latest", "updated":
		return "updated_at"
	case "rating", "popular", "popularity":
		// No rating column exists; rating falls back to popularity.
		return "popularity"
	default:
		return "updated_at"
	}
}

// MediaWriteRequest is the create/update payload for a media entry. Genre/
// author/artist taxonomies are provided as display names (the service upserts
// them and rewrites the media's join rows). SubType is a single classifier
// validated against Type via the managed sub_type vocabulary. On create, Type
// defaults to MediaManga when empty.
type MediaWriteRequest struct {
	SourceID string    `json:"sourceId"`
	Type     MediaType `json:"type,omitempty"`
	SubType  string    `json:"subType,omitempty"`
	// URL identifies the entry in the Mihon source contract. Optional on write:
	// when omitted, the service defaults it to the generated media id.
	URL         string      `json:"url,omitempty"`
	Title       string      `json:"title"`
	CoverURL    string      `json:"coverUrl,omitempty"`
	Description string      `json:"description,omitempty"`
	Status      MediaStatus `json:"status,omitempty"`
	Genres      []string    `json:"genres,omitempty"`
	Authors     []string    `json:"authors,omitempty"`
	Artists     []string    `json:"artists,omitempty"`
}

// ChapterNeighbors is the previous/next chapter around a given chapter, ordered
// by chapter number. Either side is null at the ends of the list.
type ChapterNeighbors struct {
	Previous *Chapter `json:"previous"`
	Next     *Chapter `json:"next"`
}

// ChapterWriteRequest is the create/update payload for a chapter.
type ChapterWriteRequest struct {
	MediaID    string    `json:"mediaId"`
	URL        string    `json:"url"`
	Name       string    `json:"name"`
	Number     float64   `json:"number"`
	Scanlator  string    `json:"scanlator,omitempty"`
	DateUpload time.Time `json:"dateUpload,omitempty"`
	Format     string    `json:"format,omitempty"`
}

// StoredPage is a page as persisted (R2 object key, not a public URL). The
// service turns R2Key into a fetchable ImageURL when building a Page. Kind is
// PageKindImage (or "") for AVIF pages and PageKindVideo for an HLS playlist.
type StoredPage struct {
	Index  int
	R2Key  string
	Width  int
	Height int
	Kind   string
}

// AdminPage is a chapter page as seen by the admin surface. Unlike the reader's
// Page, it deliberately exposes the raw R2 object key (R2Key) alongside a
// short-lived presigned fetch URL (ImageURL) so an operator can inspect and
// delete individual artifacts.
type AdminPage struct {
	Index    int    `json:"index"`
	R2Key    string `json:"r2Key"`
	ImageURL string `json:"imageUrl"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Kind     string `json:"kind,omitempty"`
}
