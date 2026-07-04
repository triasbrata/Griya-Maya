package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// searchLimitDefault / searchLimitMax bound the number of provider results.
const (
	searchLimitDefault = 10
	searchLimitMax     = 25
)

// Search queries a connection's external provider (MyAnimeList) and returns
// normalized media suggestions for the admin create-media autocomplete. It
// loads the connection, decrypts its access token, calls the provider's
// resource API for the endpoint matching kind, and normalizes the payload. On a
// provider 401 it refreshes the token via the existing plumbing and retries the
// search exactly once.
func (s *ConnectionService) Search(ctx context.Context, id, query, kind string, limit int) ([]domain.MediaSuggestion, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("%w: q is required", domain.ErrInvalidInput)
	}

	mediaType, err := searchMediaType(kind)
	if err != nil {
		return nil, err
	}
	limit = clampSearchLimit(limit)

	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	endpoints, ok := c.Provider.Endpoints()
	if !ok || endpoints.APIBaseURL == "" {
		return nil, fmt.Errorf("%w: provider %q has no search API", domain.ErrInvalidInput, c.Provider)
	}

	reqURL := malSearchURL(endpoints.APIBaseURL, mediaType, query, limit)

	access, err := decrypt(s.encKey, c.AccessToken)
	if err != nil {
		return nil, err
	}

	body, status, err := s.oauth.Get(ctx, reqURL, access)
	if err != nil {
		return nil, err
	}
	// Refresh-on-401 and retry once: the stored access token may have expired.
	if status == http.StatusUnauthorized {
		refreshed, rerr := s.Refresh(ctx, id)
		if rerr != nil {
			return nil, rerr
		}
		access, err = decrypt(s.encKey, refreshed.AccessToken)
		if err != nil {
			return nil, err
		}
		body, status, err = s.oauth.Get(ctx, reqURL, access)
		if err != nil {
			return nil, err
		}
	}
	if status == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: connection is not authorized", domain.ErrUnauthorized)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("provider search: unexpected status %d", status)
	}

	var payload malSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("provider search decode: %w", err)
	}

	suggestions := make([]domain.MediaSuggestion, 0, len(payload.Data))
	for _, item := range payload.Data {
		suggestions = append(suggestions, normalizeMALNode(item.Node, mediaType))
	}
	return suggestions, nil
}

// searchMediaType validates/defaults the requested kind. Empty defaults to
// manga; unknown kinds are rejected as invalid input.
func searchMediaType(kind string) (domain.MediaType, error) {
	switch domain.MediaType(kind) {
	case "":
		return domain.MediaManga, nil
	case domain.MediaManga, domain.MediaVideo, domain.MediaNovel:
		return domain.MediaType(kind), nil
	default:
		return "", fmt.Errorf("%w: unknown type %q", domain.ErrInvalidInput, kind)
	}
}

// clampSearchLimit applies the default and the 1..25 bounds.
func clampSearchLimit(limit int) int {
	if limit <= 0 {
		limit = searchLimitDefault
	}
	if limit > searchLimitMax {
		limit = searchLimitMax
	}
	return limit
}

// malSearchURL builds the MyAnimeList resource URL for a search. manga and novel
// both hit the /manga endpoint (light novels are manga on MAL); video hits
// /anime. The requested fields differ (authors for manga, studios for anime).
func malSearchURL(apiBase string, mediaType domain.MediaType, query string, limit int) string {
	path := "manga"
	fields := "id,title,synopsis,main_picture,status,genres,authors{first_name,last_name},media_type"
	if mediaType == domain.MediaVideo {
		path = "anime"
		fields = "id,title,synopsis,main_picture,status,genres,studios,media_type"
	}
	return fmt.Sprintf("%s/%s?q=%s&limit=%d&fields=%s",
		strings.TrimRight(apiBase, "/"), path, url.QueryEscape(query), limit, url.QueryEscape(fields))
}

// malSearchResponse is the MyAnimeList list envelope: an array of {node}.
type malSearchResponse struct {
	Data []struct {
		Node malNode `json:"node"`
	} `json:"data"`
}

// malNode is one MAL manga/anime record (only the requested fields).
type malNode struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Synopsis    string `json:"synopsis"`
	MainPicture struct {
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"main_picture"`
	Status string `json:"status"`
	Genres []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Authors []struct {
		Node struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"node"`
		Role string `json:"role"`
	} `json:"authors"`
	Studios []struct {
		Name string `json:"name"`
	} `json:"studios"`
	MediaType string `json:"media_type"`
}

// normalizeMALNode maps a MAL record to the camelCase MediaSuggestion contract.
// The four taxonomy slices are always non-nil so they serialize as [] not null.
func normalizeMALNode(n malNode, mediaType domain.MediaType) domain.MediaSuggestion {
	isAnime := mediaType == domain.MediaVideo

	cover := n.MainPicture.Large
	if cover == "" {
		cover = n.MainPicture.Medium
	}

	genres := make([]string, 0, len(n.Genres))
	for _, g := range n.Genres {
		if g.Name != "" {
			genres = append(genres, g.Name)
		}
	}

	authors := []string{}
	artists := []string{}
	if isAnime {
		// Anime: studios become authors; there are no artists.
		for _, st := range n.Studios {
			if st.Name != "" {
				authors = append(authors, st.Name)
			}
		}
	} else {
		// Manga/novel: split MAL author roles. "Story" → authors, "Art" →
		// artists, "Story & Art" → both, empty/other → authors.
		for _, a := range n.Authors {
			name := strings.TrimSpace(a.Node.FirstName + " " + a.Node.LastName)
			if name == "" {
				continue
			}
			isArt := strings.Contains(a.Role, "Art")
			isStory := strings.Contains(a.Role, "Story")
			switch {
			case isArt && isStory:
				authors = append(authors, name)
				artists = append(artists, name)
			case isArt:
				artists = append(artists, name)
			default:
				// "Story" or any other/empty role defaults to author.
				authors = append(authors, name)
			}
		}
	}

	urlPath := "manga"
	if isAnime {
		urlPath = "anime"
	}

	return domain.MediaSuggestion{
		ExternalID:  strconv.Itoa(n.ID),
		Title:       n.Title,
		Description: n.Synopsis,
		CoverURL:    cover,
		Status:      domain.MALStatus(n.Status, isAnime),
		Genres:      genres,
		Categories:  []string{},
		Authors:     authors,
		Artists:     artists,
		URL:         fmt.Sprintf("https://myanimelist.net/%s/%d", urlPath, n.ID),
	}
}
