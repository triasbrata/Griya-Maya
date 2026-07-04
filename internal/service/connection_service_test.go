package service

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

const fixedNow = int64(1700000000)

func newConnSvc(t *testing.T) (*ConnectionService, *mocks.MockConnectionRepository, *mocks.MockOAuthClient, *mocks.MockStateStore) {
	t.Helper()
	repo := mocks.NewMockConnectionRepository(t)
	oauth := mocks.NewMockOAuthClient(t)
	state := mocks.NewMockStateStore(t)
	svc := NewConnectionService(repo, oauth, state, testKey)
	svc.now = func() time.Time { return time.Unix(fixedNow, 0) }
	return svc, repo, oauth, state
}

func TestConnectionService_Create_EncryptsSecret(t *testing.T) {
	svc, repo, _, _ := newConnSvc(t)
	ctx := context.Background()

	var stored domain.Connection
	repo.EXPECT().Create(ctx, mock.Anything).RunAndReturn(func(_ context.Context, c domain.Connection) error {
		stored = c
		return nil
	})

	got, err := svc.Create(ctx, domain.ConnectionWriteRequest{
		Provider: domain.ProviderMyAnimeList, Label: "My MAL",
		ClientID: "abc123", ClientSecret: "topsecret",
	})
	require.NoError(t, err)

	assert.NotEmpty(t, stored.ID)
	assert.Equal(t, domain.ConnectionDisconnected, stored.Status)
	assert.Equal(t, "abc123", stored.ClientID) // client_id stays plaintext
	assert.Equal(t, fixedNow, stored.CreatedAt)
	assert.NotEqual(t, "topsecret", stored.ClientSecret, "secret must be encrypted at rest")

	// The stored ciphertext must decrypt back to the plaintext.
	plain, err := decrypt(testKey, stored.ClientSecret)
	require.NoError(t, err)
	assert.Equal(t, "topsecret", plain)

	// The returned connection redacts nothing extra and echoes the id.
	assert.Equal(t, stored.ID, got.ID)
}

func TestConnectionService_Create_Validation(t *testing.T) {
	svc, _, _, _ := newConnSvc(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, domain.ConnectionWriteRequest{Provider: "imdb", ClientID: "x", ClientSecret: "y"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = svc.Create(ctx, domain.ConnectionWriteRequest{Provider: domain.ProviderMyAnimeList, ClientSecret: "y"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = svc.Create(ctx, domain.ConnectionWriteRequest{Provider: domain.ProviderMyAnimeList, ClientID: "x"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestConnectionService_Create_RepoError(t *testing.T) {
	svc, repo, _, _ := newConnSvc(t)
	ctx := context.Background()
	wantErr := errors.New("db down")
	repo.EXPECT().Create(ctx, mock.Anything).Return(wantErr)

	_, err := svc.Create(ctx, domain.ConnectionWriteRequest{
		Provider: domain.ProviderMyAnimeList, ClientID: "x", ClientSecret: "y",
	})
	assert.ErrorIs(t, err, wantErr)
}

func TestConnectionService_ListGetDelete(t *testing.T) {
	svc, repo, _, _ := newConnSvc(t)
	ctx := context.Background()

	repo.EXPECT().List(ctx).Return([]domain.Connection{{ID: "c1"}}, nil)
	list, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	repo.EXPECT().Get(ctx, "c1").Return(domain.Connection{ID: "c1"}, nil)
	got, err := svc.Get(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, "c1", got.ID)

	repo.EXPECT().Delete(ctx, "c1").Return(nil)
	require.NoError(t, svc.Delete(ctx, "c1"))
}

func TestConnectionService_Update_ReencryptsSecret(t *testing.T) {
	svc, repo, _, _ := newConnSvc(t)
	ctx := context.Background()

	existing := domain.Connection{ID: "c1", ClientID: "old", ClientSecret: "old-enc", Label: "Old"}
	repo.EXPECT().Get(ctx, "c1").Return(existing, nil)

	var saved domain.Connection
	repo.EXPECT().Update(ctx, mock.Anything).RunAndReturn(func(_ context.Context, c domain.Connection) error {
		saved = c
		return nil
	})

	_, err := svc.Update(ctx, "c1", domain.ConnectionWriteRequest{Label: "New", ClientID: "new", ClientSecret: "fresh"})
	require.NoError(t, err)
	assert.Equal(t, "New", saved.Label)
	assert.Equal(t, "new", saved.ClientID)
	plain, err := decrypt(testKey, saved.ClientSecret)
	require.NoError(t, err)
	assert.Equal(t, "fresh", plain)
	assert.Equal(t, fixedNow, saved.UpdatedAt)
}

func TestConnectionService_Update_KeepsClientFieldsWhenEmpty(t *testing.T) {
	svc, repo, _, _ := newConnSvc(t)
	ctx := context.Background()

	existing := domain.Connection{ID: "c1", ClientID: "old", ClientSecret: "old-enc", Label: "Old"}
	repo.EXPECT().Get(ctx, "c1").Return(existing, nil)

	var saved domain.Connection
	repo.EXPECT().Update(ctx, mock.Anything).RunAndReturn(func(_ context.Context, c domain.Connection) error {
		saved = c
		return nil
	})

	_, err := svc.Update(ctx, "c1", domain.ConnectionWriteRequest{Label: ""})
	require.NoError(t, err)
	assert.Equal(t, "", saved.Label)          // label always overwritten
	assert.Equal(t, "old", saved.ClientID)    // unchanged
	assert.Equal(t, "old-enc", saved.ClientSecret) // unchanged
}

func TestConnectionService_Update_NotFound(t *testing.T) {
	svc, repo, _, _ := newConnSvc(t)
	ctx := context.Background()
	repo.EXPECT().Get(ctx, "missing").Return(domain.Connection{}, domain.ErrNotFound)
	_, err := svc.Update(ctx, "missing", domain.ConnectionWriteRequest{})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestConnectionService_Authorize(t *testing.T) {
	svc, repo, _, state := newConnSvc(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, "c1").Return(domain.Connection{
		ID: "c1", Provider: domain.ProviderMyAnimeList, ClientID: "abc123",
	}, nil)

	var savedState string
	var savedAuth domain.AuthState
	state.EXPECT().Put(ctx, mock.Anything, mock.Anything, stateTTLSeconds).
		RunAndReturn(func(_ context.Context, st string, v domain.AuthState, _ int) error {
			savedState = st
			savedAuth = v
			return nil
		})

	authURL, err := svc.Authorize(ctx, "c1", "https://admin.example/callback")
	require.NoError(t, err)

	// PKCE verifier: crypto-random, plain challenge (challenge == verifier), 43..128 chars.
	assert.GreaterOrEqual(t, len(savedAuth.CodeVerifier), 43)
	assert.LessOrEqual(t, len(savedAuth.CodeVerifier), 128)
	assert.Equal(t, "c1", savedAuth.ConnectionID)
	assert.Equal(t, "https://admin.example/callback", savedAuth.RedirectURI)
	assert.Equal(t, domain.ProviderMyAnimeList, savedAuth.Provider)

	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "myanimelist.net", u.Host)
	q := u.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "abc123", q.Get("client_id"))
	assert.Equal(t, "plain", q.Get("code_challenge_method"))
	assert.Equal(t, savedAuth.CodeVerifier, q.Get("code_challenge")) // plain: challenge == verifier
	assert.Equal(t, savedState, q.Get("state"))
	assert.Equal(t, "https://admin.example/callback", q.Get("redirect_uri"))
}

func TestConnectionService_Authorize_Errors(t *testing.T) {
	svc, repo, _, _ := newConnSvc(t)
	ctx := context.Background()

	_, err := svc.Authorize(ctx, "c1", "")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	repo.EXPECT().Get(ctx, "missing").Return(domain.Connection{}, domain.ErrNotFound)
	_, err = svc.Authorize(ctx, "missing", "https://cb")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestConnectionService_Callback(t *testing.T) {
	svc, repo, oauth, state := newConnSvc(t)
	ctx := context.Background()

	encSecret, err := encrypt(testKey, "topsecret")
	require.NoError(t, err)

	state.EXPECT().Get(ctx, "st1").Return(domain.AuthState{
		ConnectionID: "c1", CodeVerifier: "verifier-xyz",
		RedirectURI: "https://cb", Provider: domain.ProviderMyAnimeList,
	}, nil)
	repo.EXPECT().Get(ctx, "c1").Return(domain.Connection{
		ID: "c1", Provider: domain.ProviderMyAnimeList, ClientID: "abc123", ClientSecret: encSecret,
	}, nil)
	// Exchange must receive the DECRYPTED secret and PKCE verifier.
	oauth.EXPECT().Exchange(ctx, domain.ProviderMyAnimeList, "abc123", "topsecret", "code-1", "verifier-xyz", "https://cb").
		Return(domain.TokenResponse{AccessToken: "acc", RefreshToken: "ref", TokenType: "Bearer", ExpiresIn: 3600}, nil)

	var gotAccess, gotRefresh, gotType string
	var gotExpires, gotUpdated int64
	var gotStatus domain.ConnectionStatus
	repo.EXPECT().SaveTokens(ctx, "c1", mock.Anything, mock.Anything, "Bearer", mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _, access, refresh, tokenType string, expiresAt int64, status domain.ConnectionStatus, updatedAt int64) error {
			gotAccess, gotRefresh, gotType = access, refresh, tokenType
			gotExpires, gotUpdated, gotStatus = expiresAt, updatedAt, status
			return nil
		})

	got, err := svc.Callback(ctx, "code-1", "st1")
	require.NoError(t, err)
	assert.Equal(t, domain.ConnectionConnected, got.Status)
	assert.Equal(t, domain.ConnectionConnected, gotStatus)
	assert.Equal(t, "Bearer", gotType)
	assert.Equal(t, fixedNow+3600, gotExpires)
	assert.Equal(t, fixedNow, gotUpdated)

	// Tokens are stored encrypted; decrypt to confirm round-trip.
	assert.NotEqual(t, "acc", gotAccess)
	a, err := decrypt(testKey, gotAccess)
	require.NoError(t, err)
	assert.Equal(t, "acc", a)
	r, err := decrypt(testKey, gotRefresh)
	require.NoError(t, err)
	assert.Equal(t, "ref", r)
}

func TestConnectionService_Callback_Errors(t *testing.T) {
	svc, repo, oauth, state := newConnSvc(t)
	ctx := context.Background()

	_, err := svc.Callback(ctx, "", "st")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	state.EXPECT().Get(ctx, "bad").Return(domain.AuthState{}, domain.ErrNotFound).Once()
	_, err = svc.Callback(ctx, "code", "bad")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Exchange failure propagates.
	state.EXPECT().Get(ctx, "st2").Return(domain.AuthState{ConnectionID: "c1", Provider: domain.ProviderMyAnimeList}, nil).Once()
	repo.EXPECT().Get(ctx, "c1").Return(domain.Connection{ID: "c1", Provider: domain.ProviderMyAnimeList}, nil).Once()
	exErr := errors.New("exchange failed")
	oauth.EXPECT().Exchange(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(domain.TokenResponse{}, exErr).Once()
	_, err = svc.Callback(ctx, "code", "st2")
	assert.ErrorIs(t, err, exErr)
}

func TestConnectionService_Refresh(t *testing.T) {
	svc, repo, oauth, _ := newConnSvc(t)
	ctx := context.Background()

	encSecret, _ := encrypt(testKey, "topsecret")
	encRefresh, _ := encrypt(testKey, "old-refresh")
	repo.EXPECT().Get(ctx, "c1").Return(domain.Connection{
		ID: "c1", Provider: domain.ProviderMyAnimeList, ClientID: "abc123",
		ClientSecret: encSecret, RefreshToken: encRefresh, Status: domain.ConnectionConnected,
	}, nil)
	oauth.EXPECT().Refresh(ctx, domain.ProviderMyAnimeList, "abc123", "topsecret", "old-refresh").
		Return(domain.TokenResponse{AccessToken: "acc2", RefreshToken: "ref2", TokenType: "Bearer", ExpiresIn: 1200}, nil)

	var gotExpires int64
	repo.EXPECT().SaveTokens(ctx, "c1", mock.Anything, mock.Anything, "Bearer", mock.Anything, domain.ConnectionConnected, mock.Anything).
		RunAndReturn(func(_ context.Context, _, _, _, _ string, expiresAt int64, _ domain.ConnectionStatus, _ int64) error {
			gotExpires = expiresAt
			return nil
		})

	got, err := svc.Refresh(ctx, "c1")
	require.NoError(t, err)
	assert.Equal(t, domain.ConnectionConnected, got.Status)
	assert.Equal(t, fixedNow+1200, gotExpires)
}

func TestConnectionService_Refresh_Errors(t *testing.T) {
	svc, repo, oauth, _ := newConnSvc(t)
	ctx := context.Background()

	// No refresh token stored.
	repo.EXPECT().Get(ctx, "c1").Return(domain.Connection{ID: "c1", Provider: domain.ProviderMyAnimeList}, nil).Once()
	_, err := svc.Refresh(ctx, "c1")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	// Provider refresh call fails.
	encRefresh, _ := encrypt(testKey, "r")
	repo.EXPECT().Get(ctx, "c2").Return(domain.Connection{
		ID: "c2", Provider: domain.ProviderMyAnimeList, RefreshToken: encRefresh,
	}, nil).Once()
	refErr := errors.New("refresh failed")
	oauth.EXPECT().Refresh(ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(domain.TokenResponse{}, refErr).Once()
	_, err = svc.Refresh(ctx, "c2")
	assert.ErrorIs(t, err, refErr)

	// Not found.
	repo.EXPECT().Get(ctx, "missing").Return(domain.Connection{}, domain.ErrNotFound).Once()
	_, err = svc.Refresh(ctx, "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
