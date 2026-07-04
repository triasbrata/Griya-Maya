package oauth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// rtFunc adapts a function to an http.RoundTripper so a test can intercept the
// outbound token request without hitting the network.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// newClientWithRT builds a Client whose transport is the given RoundTripper.
func newClientWithRT(rt http.RoundTripper) *Client {
	return &Client{http: &http.Client{Transport: rt}}
}

func TestClient_Exchange_SendsCredentialsInBody(t *testing.T) {
	var gotForm url.Values
	var gotAuth string
	var gotURL string
	c := newClientWithRT(rtFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		gotURL = r.URL.String()
		b, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(b))
		return jsonResp(http.StatusOK,
			`{"token_type":"Bearer","expires_in":3600,"access_token":"acc","refresh_token":"ref"}`), nil
	}))

	tok, err := c.Exchange(context.Background(), domain.ProviderMyAnimeList,
		"client-123", "secret-xyz", "the-code", "the-verifier", "https://cb")
	require.NoError(t, err)

	// Credentials + PKCE params travel in the BODY (MAL "scheme 2").
	assert.Equal(t, "client-123", gotForm.Get("client_id"))
	assert.Equal(t, "secret-xyz", gotForm.Get("client_secret"))
	assert.Equal(t, "authorization_code", gotForm.Get("grant_type"))
	assert.Equal(t, "the-code", gotForm.Get("code"))
	assert.Equal(t, "the-verifier", gotForm.Get("code_verifier"))
	assert.Equal(t, "https://cb", gotForm.Get("redirect_uri"))
	// No HTTP Basic header — single, clean body scheme.
	assert.Empty(t, gotAuth)
	// Hits MAL's token endpoint.
	assert.Equal(t, "https://myanimelist.net/v1/oauth2/token", gotURL)

	assert.Equal(t, domain.TokenResponse{
		AccessToken: "acc", RefreshToken: "ref", TokenType: "Bearer", ExpiresIn: 3600,
	}, tok)
}

func TestClient_Refresh_SendsCredentialsInBody(t *testing.T) {
	var gotForm url.Values
	var gotAuth string
	c := newClientWithRT(rtFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(b))
		return jsonResp(http.StatusOK,
			`{"token_type":"Bearer","expires_in":1200,"access_token":"a2","refresh_token":"r2"}`), nil
	}))

	tok, err := c.Refresh(context.Background(), domain.ProviderMyAnimeList,
		"client-123", "secret-xyz", "old-refresh")
	require.NoError(t, err)

	assert.Equal(t, "client-123", gotForm.Get("client_id"))
	assert.Equal(t, "secret-xyz", gotForm.Get("client_secret"))
	assert.Equal(t, "refresh_token", gotForm.Get("grant_type"))
	assert.Equal(t, "old-refresh", gotForm.Get("refresh_token"))
	assert.Empty(t, gotAuth)
	assert.Equal(t, int64(1200), tok.ExpiresIn)
}

func TestClient_Exchange_Non2xxSurfacesMALError(t *testing.T) {
	c := newClientWithRT(rtFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusBadRequest, `{"error":"invalid_request","message":"bad code"}`), nil
	}))

	_, err := c.Exchange(context.Background(), domain.ProviderMyAnimeList,
		"id", "sec", "code", "verifier", "https://cb")
	require.Error(t, err)
	// The MAL status + body are included so the admin's callback Response shows the real reason.
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "invalid_request")
}

func TestClient_UnknownProvider(t *testing.T) {
	c := NewClient()
	_, err := c.Exchange(context.Background(), domain.Provider("imdb"), "id", "sec", "code", "verifier", "https://cb")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}
