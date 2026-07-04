package d1

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/repository/d1/mocks"
)

func connectionRow(id string) map[string]any {
	return map[string]any{
		"id": id, "provider": "myanimelist", "label": "My MAL",
		"client_id": "abc123", "client_secret": "enc-secret",
		"access_token": "enc-access", "refresh_token": "enc-refresh",
		"token_type": "Bearer", "expires_at": float64(1700000100),
		"status": "connected", "created_at": float64(1700000000),
		"updated_at": float64(1700000050),
	}
}

func TestConnectionRepo_Create(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &ConnectionRepo{db: q}

	var params []any
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO connection"), anyN(12)...).
		RunAndReturn(func(_ context.Context, _ string, p ...any) error {
			params = p
			return nil
		})

	c := domain.Connection{
		ID: "c1", Provider: domain.ProviderMyAnimeList, Label: "My MAL",
		ClientID: "abc123", ClientSecret: "enc-secret", Status: domain.ConnectionDisconnected,
		CreatedAt: 1700000000, UpdatedAt: 1700000000,
	}
	require.NoError(t, repo.Create(context.Background(), c))
	require.Len(t, params, 12)
	assert.Equal(t, "c1", params[0])
	assert.Equal(t, "myanimelist", params[1]) // provider stringified
	assert.Equal(t, "enc-secret", params[4])
	assert.Equal(t, "disconnected", params[9]) // status stringified
}

func TestConnectionRepo_List(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &ConnectionRepo{db: q}

	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT")).
		Return([]map[string]any{connectionRow("c1"), connectionRow("c2")}, nil)

	got, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "c1", got[0].ID)
	assert.Equal(t, domain.ProviderMyAnimeList, got[0].Provider)
	assert.Equal(t, int64(1700000100), got[0].ExpiresAt)
	assert.Equal(t, domain.ConnectionConnected, got[0].Status)
}

func TestConnectionRepo_List_Error(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &ConnectionRepo{db: q}
	wantErr := errors.New("boom")
	q.EXPECT().Query(mock.Anything, mock.Anything).Return(nil, wantErr)
	_, err := repo.List(context.Background())
	assert.ErrorIs(t, err, wantErr)
}

func TestConnectionRepo_Get_FoundAndNotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &ConnectionRepo{db: q}

	q.EXPECT().Query(mock.Anything, mock.Anything, "c1").
		Return([]map[string]any{connectionRow("c1")}, nil).Once()
	got, err := repo.Get(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "abc123", got.ClientID)
	assert.Equal(t, "enc-secret", got.ClientSecret)

	q.EXPECT().Query(mock.Anything, mock.Anything, "missing").Return(nil, nil).Once()
	_, err = repo.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestConnectionRepo_Update(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &ConnectionRepo{db: q}

	var params []any
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE connection"), anyN(5)...).
		RunAndReturn(func(_ context.Context, _ string, p ...any) error {
			params = p
			return nil
		})

	c := domain.Connection{ID: "c1", Label: "New", ClientID: "id2", ClientSecret: "enc2", UpdatedAt: 1700000200}
	require.NoError(t, repo.Update(context.Background(), c))
	require.Len(t, params, 5)
	assert.Equal(t, "c1", params[0])
	assert.Equal(t, "New", params[1])
	assert.Equal(t, "enc2", params[3])
}

func TestConnectionRepo_Delete(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &ConnectionRepo{db: q}
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM connection"), "c1").Return(nil)
	require.NoError(t, repo.Delete(context.Background(), "c1"))
}

func TestConnectionRepo_SaveTokens(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &ConnectionRepo{db: q}

	var params []any
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE connection SET access_token"), anyN(7)...).
		RunAndReturn(func(_ context.Context, _ string, p ...any) error {
			params = p
			return nil
		})

	err := repo.SaveTokens(context.Background(), "c1", "enc-a", "enc-r", "Bearer", 1700009999, domain.ConnectionConnected, 1700000300)
	require.NoError(t, err)
	require.Len(t, params, 7)
	assert.Equal(t, "c1", params[0])
	assert.Equal(t, "enc-a", params[1])
	assert.Equal(t, "enc-r", params[2])
	assert.Equal(t, "Bearer", params[3])
	assert.Equal(t, int64(1700009999), params[4])
	assert.Equal(t, "connected", params[5])
}
