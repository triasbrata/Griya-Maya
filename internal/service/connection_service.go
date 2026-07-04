package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// ConnectionService manages external-source OAuth connections: CRUD plus the
// authorize → callback → refresh token flow. Secrets and tokens are encrypted
// before they reach the repository and decrypted only in-process when needed to
// talk to the provider, so plaintext never leaves this layer.
type ConnectionService struct {
	repo   ConnectionRepository
	oauth  OAuthClient
	state  StateStore
	encKey []byte
	now    func() time.Time
}

// NewConnectionService wires a ConnectionService. encKey must be 32 bytes
// (AES-256); binding validates this at startup.
func NewConnectionService(repo ConnectionRepository, oauth OAuthClient, state StateStore, encKey []byte) *ConnectionService {
	return &ConnectionService{repo: repo, oauth: oauth, state: state, encKey: encKey, now: time.Now}
}

// Create validates and stores a new connection with its client secret encrypted
// at rest. It starts life disconnected (no tokens yet).
func (s *ConnectionService) Create(ctx context.Context, req domain.ConnectionWriteRequest) (domain.Connection, error) {
	if !req.Provider.Valid() {
		return domain.Connection{}, fmt.Errorf("%w: unknown provider", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(req.ClientID) == "" {
		return domain.Connection{}, fmt.Errorf("%w: client_id is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(req.ClientSecret) == "" {
		return domain.Connection{}, fmt.Errorf("%w: client_secret is required", domain.ErrInvalidInput)
	}

	encSecret, err := encrypt(s.encKey, req.ClientSecret)
	if err != nil {
		return domain.Connection{}, err
	}

	now := s.now().Unix()
	c := domain.Connection{
		ID:           uuid.NewString(),
		Provider:     req.Provider,
		Label:        strings.TrimSpace(req.Label),
		ClientID:     strings.TrimSpace(req.ClientID),
		ClientSecret: encSecret,
		Status:       domain.ConnectionDisconnected,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return domain.Connection{}, err
	}
	return c, nil
}

// List returns all connections (secrets/tokens are dropped from the JSON).
func (s *ConnectionService) List(ctx context.Context) ([]domain.Connection, error) {
	return s.repo.List(ctx)
}

// Get returns one connection or domain.ErrNotFound.
func (s *ConnectionService) Get(ctx context.Context, id string) (domain.Connection, error) {
	return s.repo.Get(ctx, id)
}

// Update changes label and/or client credentials. Empty client_id/client_secret
// leave the stored values unchanged; label is always overwritten.
func (s *ConnectionService) Update(ctx context.Context, id string, req domain.ConnectionWriteRequest) (domain.Connection, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Connection{}, err
	}
	c.Label = strings.TrimSpace(req.Label)
	if v := strings.TrimSpace(req.ClientID); v != "" {
		c.ClientID = v
	}
	if v := strings.TrimSpace(req.ClientSecret); v != "" {
		enc, err := encrypt(s.encKey, v)
		if err != nil {
			return domain.Connection{}, err
		}
		c.ClientSecret = enc
	}
	c.UpdatedAt = s.now().Unix()
	if err := s.repo.Update(ctx, c); err != nil {
		return domain.Connection{}, err
	}
	return c, nil
}

// Delete removes a connection.
func (s *ConnectionService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Authorize starts the PKCE flow: it persists a fresh state/verifier and returns
// the provider's authorize URL the caller should redirect the user to.
func (s *ConnectionService) Authorize(ctx context.Context, id, redirectURI string) (string, error) {
	if strings.TrimSpace(redirectURI) == "" {
		return "", fmt.Errorf("%w: redirect_uri is required", domain.ErrInvalidInput)
	}
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}

	// PKCE "plain": code_challenge == code_verifier (MyAnimeList's only mode).
	verifier, err := randomURLSafe(64)
	if err != nil {
		return "", err
	}
	state, err := randomURLSafe(32)
	if err != nil {
		return "", err
	}

	if err := s.state.Put(ctx, state, domain.AuthState{
		ConnectionID: c.ID,
		CodeVerifier: verifier,
		RedirectURI:  redirectURI,
		Provider:     c.Provider,
	}, stateTTLSeconds); err != nil {
		return "", err
	}

	// client_id is not a secret and is stored in plaintext.
	return authorizeURL(c.Provider, c.ClientID, verifier, state, redirectURI)
}

// Callback completes the flow: it resolves the stored state, exchanges the code
// for tokens, encrypts them, and marks the connection connected.
func (s *ConnectionService) Callback(ctx context.Context, code, state string) (domain.Connection, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(state) == "" {
		return domain.Connection{}, fmt.Errorf("%w: code and state are required", domain.ErrInvalidInput)
	}
	as, err := s.state.Get(ctx, state)
	if err != nil {
		return domain.Connection{}, err
	}
	c, err := s.repo.Get(ctx, as.ConnectionID)
	if err != nil {
		return domain.Connection{}, err
	}
	secret, err := decrypt(s.encKey, c.ClientSecret)
	if err != nil {
		return domain.Connection{}, err
	}

	tok, err := s.oauth.Exchange(ctx, c.Provider, c.ClientID, secret, code, as.CodeVerifier, as.RedirectURI)
	if err != nil {
		return domain.Connection{}, err
	}
	return s.persistTokens(ctx, c, tok)
}

// Refresh renews tokens for an already-connected connection.
func (s *ConnectionService) Refresh(ctx context.Context, id string) (domain.Connection, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Connection{}, err
	}
	secret, err := decrypt(s.encKey, c.ClientSecret)
	if err != nil {
		return domain.Connection{}, err
	}
	refresh, err := decrypt(s.encKey, c.RefreshToken)
	if err != nil {
		return domain.Connection{}, err
	}
	if refresh == "" {
		return domain.Connection{}, fmt.Errorf("%w: no refresh token to renew", domain.ErrInvalidInput)
	}

	tok, err := s.oauth.Refresh(ctx, c.Provider, c.ClientID, secret, refresh)
	if err != nil {
		return domain.Connection{}, err
	}
	return s.persistTokens(ctx, c, tok)
}

// persistTokens encrypts a token response and saves it against the connection,
// moving it to connected. It returns the updated (redacted) connection.
func (s *ConnectionService) persistTokens(ctx context.Context, c domain.Connection, tok domain.TokenResponse) (domain.Connection, error) {
	encAccess, err := encrypt(s.encKey, tok.AccessToken)
	if err != nil {
		return domain.Connection{}, err
	}
	encRefresh, err := encrypt(s.encKey, tok.RefreshToken)
	if err != nil {
		return domain.Connection{}, err
	}
	now := s.now().Unix()
	expiresAt := now + tok.ExpiresIn
	if err := s.repo.SaveTokens(ctx, c.ID, encAccess, encRefresh, tok.TokenType, expiresAt, domain.ConnectionConnected, now); err != nil {
		return domain.Connection{}, err
	}

	c.AccessToken = encAccess
	c.RefreshToken = encRefresh
	c.TokenType = tok.TokenType
	c.ExpiresAt = expiresAt
	c.Status = domain.ConnectionConnected
	c.UpdatedAt = now
	return c, nil
}
