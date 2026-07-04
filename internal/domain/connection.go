package domain

// Provider identifies an external media source we hold OAuth credentials for. It
// is designed to grow: adding a source (e.g. "imdb") is a one-line addition to
// the const block and to the providerEndpoints table below.
type Provider string

const (
	// ProviderMyAnimeList is the MyAnimeList OAuth2 provider.
	ProviderMyAnimeList Provider = "myanimelist"
)

// Valid reports whether p is a known provider.
func (p Provider) Valid() bool {
	switch p {
	case ProviderMyAnimeList:
		return true
	}
	return false
}

// ProviderEndpoints holds the per-provider OAuth2 endpoints and PKCE challenge
// method. It is the single source of truth shared by the service (which builds
// the authorize URL) and the OAuth repository client (which POSTs to the token
// URL), so both stay in sync and IMDB is a one-line addition.
type ProviderEndpoints struct {
	AuthorizeURL string
	TokenURL     string
	// ChallengeMethod is the PKCE method: "plain" or "S256". MyAnimeList only
	// supports "plain" (code_challenge == code_verifier).
	ChallengeMethod string
}

// providerEndpoints is the registry. Add a new provider by adding one entry.
var providerEndpoints = map[Provider]ProviderEndpoints{
	ProviderMyAnimeList: {
		AuthorizeURL:    "https://myanimelist.net/v1/oauth2/authorize",
		TokenURL:        "https://myanimelist.net/v1/oauth2/token",
		ChallengeMethod: "plain",
	},
}

// Endpoints returns the OAuth endpoints for p and whether it is registered.
func (p Provider) Endpoints() (ProviderEndpoints, bool) {
	e, ok := providerEndpoints[p]
	return e, ok
}

// ConnectionStatus is the lifecycle state of a connection's OAuth link.
type ConnectionStatus string

const (
	// ConnectionDisconnected is a stored connection that has never completed (or
	// has lost) its OAuth authorization.
	ConnectionDisconnected ConnectionStatus = "disconnected"
	// ConnectionConnected is a connection holding valid tokens.
	ConnectionConnected ConnectionStatus = "connected"
	// ConnectionError marks a connection whose last token operation failed.
	ConnectionError ConnectionStatus = "error"
)

// Connection is a stored OAuth credential/token record for one external
// provider. Secrets never leave the process: ClientSecret and the tokens carry
// `json:"-"` so they are dropped from every API response (the fields hold
// AES-GCM ciphertext at rest anyway). ClientID is not a secret and is surfaced.
type Connection struct {
	ID           string           `json:"id"`
	Provider     Provider         `json:"provider"`
	Label        string           `json:"label"`
	ClientID     string           `json:"client_id"`
	ClientSecret string           `json:"-"`
	AccessToken  string           `json:"-"`
	RefreshToken string           `json:"-"`
	TokenType    string           `json:"-"`
	ExpiresAt    int64            `json:"expires_at"`
	Status       ConnectionStatus `json:"status"`
	CreatedAt    int64            `json:"created_at"`
	UpdatedAt    int64            `json:"updated_at"`
}

// Connected reports whether the connection currently holds a live authorization.
func (c Connection) Connected() bool {
	return c.Status == ConnectionConnected
}

// ConnectionWriteRequest is the create/update payload. On update the client
// fields are optional: an empty ClientID/ClientSecret leaves the stored value
// unchanged (an empty Label clears it).
type ConnectionWriteRequest struct {
	Provider     Provider `json:"provider"`
	Label        string   `json:"label"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
}

// TokenResponse is the normalized result of an OAuth token exchange or refresh.
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}

// AuthState is the PKCE/state bundle persisted between the authorize redirect
// and the callback. It is stored (keyed by the opaque `state`) in the KV-backed
// StateStore with a short TTL.
type AuthState struct {
	ConnectionID string   `json:"connectionId"`
	CodeVerifier string   `json:"codeVerifier"`
	RedirectURI  string   `json:"redirectUri"`
	Provider     Provider `json:"provider"`
}
