package d1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/repository/d1/mocks"
)

func adRow(id string, active int) map[string]any {
	return map[string]any{
		"id": id, "r2_key": "ads/" + id, "click_url": "https://x", "weight": float64(5),
		"placement": "reader_interstitial", "width": float64(600), "height": float64(300),
		"active": float64(active), "created_at": float64(1000),
	}
}

func TestAdRepo_List_All(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &AdRepo{db: q}
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT")).
		Return([]map[string]any{adRow("a", 1), adRow("b", 0)}, nil)
	got, err := repo.List(context.Background(), false, "")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, "ads/a", got[0].R2Key)
	assert.True(t, got[0].Active)
	assert.False(t, got[1].Active)
}

func TestAdRepo_List_ActiveAndPlacementFilter(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &AdRepo{db: q}
	var gotSQL string
	var gotArgs []any
	q.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sql string, args ...any) ([]map[string]any, error) {
			gotSQL, gotArgs = sql, args
			return nil, nil
		})
	_, err := repo.List(context.Background(), true, "reader_interstitial")
	require.NoError(t, err)
	assert.Contains(t, gotSQL, "active = 1")
	assert.Contains(t, gotSQL, "placement = ?1")
	assert.Contains(t, gotSQL, "ORDER BY weight DESC")
	require.Len(t, gotArgs, 1)
	assert.Equal(t, "reader_interstitial", gotArgs[0])
}

func TestAdRepo_Get(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &AdRepo{db: q}
	q.EXPECT().Query(mock.Anything, mock.Anything, "a").Return([]map[string]any{adRow("a", 1)}, nil).Once()
	got, err := repo.Get(context.Background(), "a")
	require.NoError(t, err)
	assert.Equal(t, "a", got.ID)
	assert.Equal(t, 5, got.Weight)
}

func TestAdRepo_Get_NotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &AdRepo{db: q}
	q.EXPECT().Query(mock.Anything, mock.Anything, "missing").Return(nil, nil).Once()
	_, err := repo.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAdRepo_CreateUpdateDelete(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &AdRepo{db: q}

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO ads"), anyN(9)...).Return(nil)
	require.NoError(t, repo.Create(context.Background(),
		domain.StoredAd{ID: "a", R2Key: "ads/a", Weight: 1, Active: true}))

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE ads"), anyN(8)...).Return(nil)
	require.NoError(t, repo.Update(context.Background(),
		domain.StoredAd{ID: "a", R2Key: "ads/a2", Weight: 2, Active: false}))

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM ads"), "a").Return(nil)
	require.NoError(t, repo.Delete(context.Background(), "a"))
}
