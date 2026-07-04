package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

func TestProvider_Valid(t *testing.T) {
	assert.True(t, domain.ProviderMyAnimeList.Valid())
	assert.False(t, domain.Provider("imdb").Valid())
	assert.False(t, domain.Provider("").Valid())
}

func TestProvider_Endpoints(t *testing.T) {
	e, ok := domain.ProviderMyAnimeList.Endpoints()
	require.True(t, ok)
	assert.Equal(t, "https://myanimelist.net/v1/oauth2/authorize", e.AuthorizeURL)
	assert.Equal(t, "https://myanimelist.net/v1/oauth2/token", e.TokenURL)
	assert.Equal(t, "https://api.myanimelist.net/v2", e.APIBaseURL)
	assert.Equal(t, "plain", e.ChallengeMethod)

	_, ok = domain.Provider("imdb").Endpoints()
	assert.False(t, ok)
}

func TestMALStatus_Manga(t *testing.T) {
	cases := map[string]domain.MediaStatus{
		"finished":             domain.StatusCompleted,
		"currently_publishing": domain.StatusOngoing,
		"on_hiatus":            domain.StatusHiatus,
		"discontinued":         domain.StatusCancelled,
		"not_yet_published":    domain.StatusUnknown,
		"":                     domain.StatusUnknown,
		"bogus":                domain.StatusUnknown,
		// Anime statuses must NOT match under the manga vocabulary.
		"currently_airing": domain.StatusUnknown,
	}
	for in, want := range cases {
		assert.Equalf(t, want, domain.MALStatus(in, false), "manga status %q", in)
	}
}

func TestMALStatus_Anime(t *testing.T) {
	cases := map[string]domain.MediaStatus{
		"finished_airing":  domain.StatusCompleted,
		"currently_airing": domain.StatusOngoing,
		"not_yet_aired":    domain.StatusUnknown,
		"":                 domain.StatusUnknown,
		"bogus":            domain.StatusUnknown,
		// Manga statuses must NOT match under the anime vocabulary.
		"finished":  domain.StatusUnknown,
		"on_hiatus": domain.StatusUnknown,
	}
	for in, want := range cases {
		assert.Equalf(t, want, domain.MALStatus(in, true), "anime status %q", in)
	}
}

// TestMediaSuggestion_JSONContract locks the exact camelCase field names and
// verifies the four taxonomy fields serialize as [] (never null) when non-nil.
func TestMediaSuggestion_JSONContract(t *testing.T) {
	s := domain.MediaSuggestion{
		ExternalID:  "44347",
		Title:       "One Piece",
		Description: "desc",
		CoverURL:    "https://img/large.jpg",
		Status:      domain.StatusOngoing,
		Genres:      []string{"Action"},
		Categories:  []string{},
		Authors:     []string{"Eiichiro Oda"},
		Artists:     []string{},
		URL:         "https://myanimelist.net/manga/44347",
	}
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	got := string(raw)

	for _, field := range []string{
		`"externalId":"44347"`, `"title":"One Piece"`, `"description":"desc"`,
		`"coverUrl":"https://img/large.jpg"`, `"status":"ongoing"`,
		`"genres":["Action"]`, `"categories":[]`, `"authors":["Eiichiro Oda"]`,
		`"artists":[]`, `"url":"https://myanimelist.net/manga/44347"`,
	} {
		assert.Truef(t, strings.Contains(got, field), "missing/incorrect field %s in %s", field, got)
	}
	assert.NotContains(t, got, "null")
}

func TestConnectionStatus_Consts(t *testing.T) {
	assert.Equal(t, domain.ConnectionStatus("disconnected"), domain.ConnectionDisconnected)
	assert.Equal(t, domain.ConnectionStatus("connected"), domain.ConnectionConnected)
	assert.Equal(t, domain.ConnectionStatus("error"), domain.ConnectionError)
}

func TestConnection_Connected(t *testing.T) {
	assert.True(t, domain.Connection{Status: domain.ConnectionConnected}.Connected())
	assert.False(t, domain.Connection{Status: domain.ConnectionDisconnected}.Connected())
}

// TestConnection_JSONRedaction verifies secrets and tokens never serialize, and
// that the public contract fields are all present.
func TestConnection_JSONRedaction(t *testing.T) {
	c := domain.Connection{
		ID:           "id-1",
		Provider:     domain.ProviderMyAnimeList,
		Label:        "My MAL",
		ClientID:     "abc123",
		ClientSecret: "SUPER-SECRET",
		AccessToken:  "ACCESS-TOKEN",
		RefreshToken: "REFRESH-TOKEN",
		TokenType:    "Bearer",
		ExpiresAt:    1700000000,
		Status:       domain.ConnectionDisconnected,
		CreatedAt:    1700000000,
		UpdatedAt:    1700000000,
	}
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	s := string(raw)

	for _, secret := range []string{"SUPER-SECRET", "ACCESS-TOKEN", "REFRESH-TOKEN", "Bearer"} {
		assert.NotContainsf(t, s, secret, "secret %q leaked into JSON", secret)
	}
	for _, field := range []string{`"id"`, `"provider"`, `"label"`, `"client_id"`, `"status"`, `"expires_at"`, `"created_at"`, `"updated_at"`} {
		assert.Truef(t, strings.Contains(s, field), "missing contract field %s", field)
	}
	assert.Contains(t, s, `"client_id":"abc123"`)
	assert.Contains(t, s, `"provider":"myanimelist"`)
	// The redacted fields must not appear as keys either.
	assert.NotContains(t, s, "client_secret")
	assert.NotContains(t, s, "access_token")
	assert.NotContains(t, s, "refresh_token")
	assert.NotContains(t, s, "token_type")
}
