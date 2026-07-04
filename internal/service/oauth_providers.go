package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// stateTTLSeconds bounds how long a pending authorize/callback flow stays valid.
const stateTTLSeconds = 600

// authorizeURL builds a provider's OAuth2 authorize URL for a PKCE flow. The
// endpoint table lives in domain (domain.Provider.Endpoints) so the service and
// the outbound OAuth client share one registry. MyAnimeList uses the "plain"
// challenge method, so code_challenge == code_verifier.
func authorizeURL(p domain.Provider, clientID, codeChallenge, state, redirectURI string) (string, error) {
	endpoints, ok := p.Endpoints()
	if !ok {
		return "", fmt.Errorf("%w: unknown provider %q", domain.ErrInvalidInput, p)
	}
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {endpoints.ChallengeMethod},
		"state":                 {state},
		"redirect_uri":          {redirectURI},
	}
	return endpoints.AuthorizeURL + "?" + q.Encode(), nil
}

// randomURLSafe returns n random bytes base64url-encoded (no padding), suitable
// for a PKCE code_verifier or an opaque state. It uses crypto/rand only. For a
// PKCE verifier pass n=64 → an 86-char token, within the RFC 7636 43..128 range.
func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
