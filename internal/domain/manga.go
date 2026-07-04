// Package domain holds the core entities exchanged over the API.
//
// The JSON shapes intentionally mirror the Mihon iOS `SourceRuntime` contract
// (popular/latest/search -> [Manga], details -> MangaDetails, chapters ->
// [Chapter], pages -> [Page]) so a new `engineFamily` in the app's repo.json
// can consume this server directly.
package domain

import "time"

// MangaStatus mirrors the app's status enum.
type MangaStatus string

const (
	StatusUnknown    MangaStatus = "unknown"
	StatusOngoing    MangaStatus = "ongoing"
	StatusCompleted  MangaStatus = "completed"
	StatusLicensed   MangaStatus = "licensed"
	StatusHiatus     MangaStatus = "hiatus"
	StatusCancelled  MangaStatus = "cancelled"
	StatusPublishing MangaStatus = "publishing_finished"
)

// Manga is a catalog entry (list/search item and detail base).
type Manga struct {
	ID          string      `json:"id"`
	SourceID    string      `json:"sourceId"`
	URL         string      `json:"url"`
	Title       string      `json:"title"`
	CoverURL    string      `json:"coverUrl,omitempty"`
	Author      string      `json:"author,omitempty"`
	Artist      string      `json:"artist,omitempty"`
	Description string      `json:"description,omitempty"`
	Genres      []string    `json:"genres,omitempty"`
	Status      MangaStatus `json:"status,omitempty"`
	UpdatedAt   time.Time   `json:"updatedAt,omitempty"`
}

// MangaPage is a paginated slice of catalog entries.
type MangaPage struct {
	Items   []Manga `json:"items"`
	HasNext bool    `json:"hasNext"`
	Page    int     `json:"page"`
}

// Chapter is a single chapter belonging to a manga.
type Chapter struct {
	ID         string    `json:"id"`
	MangaID    string    `json:"mangaId"`
	URL        string    `json:"url"`
	Name       string    `json:"name"`
	Number     float64   `json:"number"`
	Scanlator  string    `json:"scanlator,omitempty"`
	DateUpload time.Time `json:"dateUpload,omitempty"`
	// Format is the stored artifact type when this chapter was ingested from an
	// archive ("cbz" | "epub" | "pdf" | ""). Empty means remote-only.
	Format string `json:"format,omitempty"`
}

// Page is one readable image. ImageURL points at an AVIF object in R2
// (either a public R2 URL or a proxied URL through this service).
type Page struct {
	Index    int    `json:"index"`
	ImageURL string `json:"imageUrl"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// GenreTag is a filterable genre. The shape mirrors the app's GenreTag
// (`{slug, name}`, where the app derives its `id` from `slug`).
type GenreTag struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// GenreMode selects how multiple included genres combine.
type GenreMode string

const (
	// GenreModeOr matches manga carrying ANY of the included genres (default).
	GenreModeOr GenreMode = "OR"
	// GenreModeAnd matches only manga carrying ALL of the included genres.
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
	// Types are content-type tags (e.g. "manga" | "manhwa" | "manhua"). The
	// catalog has no dedicated type column, so these are matched against genres.
	Types []string
	// IncludeGenres / ExcludeGenres are genre slugs to require / forbid.
	IncludeGenres []string
	ExcludeGenres []string
	// GenreMode combines IncludeGenres with OR (any) or AND (all). Default OR.
	GenreMode GenreMode
}

// SortColumn maps a filter sort key to a manga column. It is the single source
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

// StoredPage is a page as persisted (R2 object key, not a public URL). The
// service turns R2Key into a fetchable ImageURL when building a Page.
type StoredPage struct {
	Index  int
	R2Key  string
	Width  int
	Height int
}
