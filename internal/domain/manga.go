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
// kind. Genres/Categories/Authors/Artists are normalized in storage and surfaced
// here as display-name arrays.
type Media struct {
	ID          string      `json:"id"`
	SourceID    string      `json:"sourceId"`
	Type        MediaType   `json:"type"`
	URL         string      `json:"url"`
	Title       string      `json:"title"`
	CoverURL    string      `json:"coverUrl,omitempty"`
	Description string      `json:"description,omitempty"`
	Genres      []string    `json:"genres,omitempty"`
	Categories  []string    `json:"categories,omitempty"`
	Authors     []string    `json:"authors,omitempty"`
	Artists     []string    `json:"artists,omitempty"`
	Status      MediaStatus `json:"status,omitempty"`
	UpdatedAt   time.Time   `json:"updatedAt,omitempty"`
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

// TaxonomyKind names one of the four normalized, managed taxonomies. It maps 1:1
// to a storage table (genre/category/author/artist) and its join table.
type TaxonomyKind string

const (
	TaxonomyGenre    TaxonomyKind = "genre"
	TaxonomyCategory TaxonomyKind = "category"
	TaxonomyAuthor   TaxonomyKind = "author"
	TaxonomyArtist   TaxonomyKind = "artist"
)

// Valid reports whether k is a known taxonomy kind.
func (k TaxonomyKind) Valid() bool {
	switch k {
	case TaxonomyGenre, TaxonomyCategory, TaxonomyAuthor, TaxonomyArtist:
		return true
	}
	return false
}

// HasSlug reports whether the taxonomy carries a slug (genre/category do;
// author/artist are name-only).
func (k TaxonomyKind) HasSlug() bool {
	return k == TaxonomyGenre || k == TaxonomyCategory
}

// Taxonomy is a normalized, managed tag: a genre, category, author, or artist.
// Slug is set only for genre/category (the app derives its filter id from it);
// it is empty (and omitted) for author/artist.
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

// GenreMode selects how multiple included genres combine.
type GenreMode string

const (
	// GenreModeOr matches media carrying ANY of the included genres (default).
	GenreModeOr GenreMode = "OR"
	// GenreModeAnd matches only media carrying ALL of the included genres.
	GenreModeAnd GenreMode = "AND"
)

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
	// IncludeGenres / ExcludeGenres are genre slugs to require / forbid.
	IncludeGenres []string
	ExcludeGenres []string
	// IncludeCategories / ExcludeCategories are category slugs to require / forbid.
	IncludeCategories []string
	ExcludeCategories []string
	// GenreMode combines IncludeGenres/IncludeCategories with OR (any) or AND
	// (all). Default OR.
	GenreMode GenreMode
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

// MediaWriteRequest is the create/update payload for a media entry. Taxonomies
// are provided as display names (the service upserts them and rewrites the
// media's join rows). On create, Type defaults to MediaManga when empty.
type MediaWriteRequest struct {
	SourceID string    `json:"sourceId"`
	Type     MediaType `json:"type,omitempty"`
	// URL identifies the entry in the Mihon source contract. Optional on write:
	// when omitted, the service defaults it to the generated media id.
	URL         string      `json:"url,omitempty"`
	Title       string      `json:"title"`
	CoverURL    string      `json:"coverUrl,omitempty"`
	Description string      `json:"description,omitempty"`
	Status      MediaStatus `json:"status,omitempty"`
	Genres      []string    `json:"genres,omitempty"`
	Categories  []string    `json:"categories,omitempty"`
	Authors     []string    `json:"authors,omitempty"`
	Artists     []string    `json:"artists,omitempty"`
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
