// Package oauth is an outbound OAuth2 client for external providers
// (MyAnimeList, later IMDB). It performs the token exchange/refresh over
// net/http and normalizes the response into domain.TokenResponse. Provider
// endpoints come from domain.Provider.Endpoints() so this adapter and the
// service share one registry.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// Client exchanges/refreshes OAuth2 tokens against a provider's token endpoint.
type Client struct {
	http *http.Client
}

// NewClient builds an OAuth client with a bounded timeout.
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 20 * time.Second}}
}

// tokenResponse is the raw provider token payload (MAL/OAuth2 standard shape).
type tokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Exchange trades an authorization code (+ PKCE verifier) for tokens.
func (c *Client) Exchange(ctx context.Context, p domain.Provider, clientID, clientSecret, code, codeVerifier, redirectURI string) (domain.TokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {codeVerifier},
		"redirect_uri":  {redirectURI},
	}
	return c.post(ctx, p, clientID, clientSecret, form)
}

// Refresh trades a refresh token for a fresh access token.
func (c *Client) Refresh(ctx context.Context, p domain.Provider, clientID, clientSecret, refreshToken string) (domain.TokenResponse, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	return c.post(ctx, p, clientID, clientSecret, form)
}

// Get performs an authenticated GET against an already-built provider resource
// URL, sending accessToken as a Bearer credential. It returns the raw response
// body together with the HTTP status code (and does not treat non-2xx as an
// error) so the caller can detect a 401 and drive a token refresh + retry.
func (c *Client) Get(ctx context.Context, url, accessToken string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("oauth get %q: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("oauth get read body: %w", err)
	}
	return body, resp.StatusCode, nil
}

// post form-encodes the request to the provider's token URL and decodes the
// token payload. Client credentials go in the request BODY (client_id +
// client_secret), not an Authorization: Basic header: MyAnimeList's
// /v1/oauth2/token is unreliable with Basic auth and expects the credentials as
// form params, so we use the body scheme exclusively (sending both can trip
// "invalid_request" on some MAL responses).
func (c *Client) post(ctx context.Context, p domain.Provider, clientID, clientSecret string, form url.Values) (domain.TokenResponse, error) {
	endpoints, ok := p.Endpoints()
	if !ok {
		return domain.TokenResponse{}, fmt.Errorf("%w: unknown provider %q", domain.ErrInvalidInput, p)
	}

	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoints.TokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return domain.TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return domain.TokenResponse{}, fmt.Errorf("oauth token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return domain.TokenResponse{}, fmt.Errorf("oauth token request: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return domain.TokenResponse{}, fmt.Errorf("oauth token decode: %w", err)
	}
	return domain.TokenResponse{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		ExpiresIn:    tr.ExpiresIn,
	}, nil
}
