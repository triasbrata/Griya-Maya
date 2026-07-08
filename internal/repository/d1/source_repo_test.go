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

func sourceRow(id string, enabled int) map[string]any {
	return map[string]any{
		"id": id, "name": "N-" + id, "lang": "en", "icon_url": "",
		"enabled": float64(enabled), "created_at": float64(1000), "updated_at": float64(1000),
	}
}

func TestSourceRepo_List(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &SourceRepo{db: q}
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT id, name")).
		Return([]map[string]any{sourceRow("a", 1), sourceRow("b", 1)}, nil)
	// Distinct media types are folded in from a single grouped scan. Source "a"
	// carries manga+novel; "b" has no media (no rows) so stays without MediaTypes.
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT source_id, type FROM media")).
		Return([]map[string]any{
			{"source_id": "a", "type": "manga"},
			{"source_id": "a", "type": "novel"},
		}, nil)
	got, err := repo.List(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.True(t, got[0].Enabled)
	assert.Equal(t, []string{"manga", "novel"}, got[0].MediaTypes)
	assert.Nil(t, got[1].MediaTypes)
}

func TestSourceRepo_List_EnabledOnlyFilters(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &SourceRepo{db: q}
	var gotSQL string
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT id, name")).RunAndReturn(
		func(_ context.Context, sql string, _ ...any) ([]map[string]any, error) {
			gotSQL = sql
			return nil, nil
		})
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT source_id, type FROM media")).
		Return(nil, nil)
	_, err := repo.List(context.Background(), true)
	require.NoError(t, err)
	assert.Contains(t, gotSQL, "enabled = 1")
}

func TestSourceRepo_Get(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &SourceRepo{db: q}
	q.EXPECT().Query(mock.Anything, mock.Anything, "a").Return([]map[string]any{sourceRow("a", 1)}, nil).Once()
	got, err := repo.Get(context.Background(), "a")
	require.NoError(t, err)
	assert.Equal(t, "a", got.ID)
}

func TestSourceRepo_Get_NotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &SourceRepo{db: q}
	q.EXPECT().Query(mock.Anything, mock.Anything, "missing").Return(nil, nil).Once()
	_, err := repo.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSourceRepo_ExistsAndMediaCount(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &SourceRepo{db: q}

	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT id FROM source"), "a").
		Return([]map[string]any{{"id": "a"}}, nil)
	ok, err := repo.Exists(context.Background(), "a")
	require.NoError(t, err)
	assert.True(t, ok)

	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT count(*)"), "a").
		Return([]map[string]any{{"n": float64(2)}}, nil)
	n, err := repo.MediaCount(context.Background(), "a")
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestSourceRepo_CreateUpdateDelete(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &SourceRepo{db: q}

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO source"), anyN(6)...).Return(nil)
	require.NoError(t, repo.Create(context.Background(),
		domain.Source{ID: "a", Name: "A", Lang: "en", Enabled: true}))

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE source"), anyN(6)...).Return(nil)
	require.NoError(t, repo.Update(context.Background(),
		domain.Source{ID: "a", Name: "A2", Enabled: false}))

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM source"), "a").Return(nil)
	require.NoError(t, repo.Delete(context.Background(), "a"))
}
