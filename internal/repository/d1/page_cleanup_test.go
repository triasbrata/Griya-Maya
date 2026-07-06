package d1

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/repository/d1/mocks"
)

func TestMediaRepo_DeletePage(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM page"), "c1", 3).Return(nil).Once()

	require.NoError(t, repo.DeletePage(context.Background(), "c1", 3))
}

func TestMediaRepo_DeletePage_Error(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	wantErr := errors.New("boom")

	q.EXPECT().Exec(mock.Anything, mock.Anything, "c1", 3).Return(wantErr).Once()

	assert.ErrorIs(t, repo.DeletePage(context.Background(), "c1", 3), wantErr)
}

func TestMediaRepo_PageKeysForMedia(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}

	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT r2_key FROM page"), "m1").
		Return([]map[string]any{
			{"r2_key": "pages/a.avif"},
			{"r2_key": ""}, // blank keys are dropped
			{"r2_key": "pages/b.avif"},
		}, nil).Once()

	keys, err := repo.PageKeysForMedia(context.Background(), "m1")
	require.NoError(t, err)
	assert.Equal(t, []string{"pages/a.avif", "pages/b.avif"}, keys)
}

func TestMediaRepo_PageKeysForMedia_Error(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	wantErr := errors.New("db down")

	q.EXPECT().Query(mock.Anything, mock.Anything, "m1").Return(nil, wantErr).Once()

	_, err := repo.PageKeysForMedia(context.Background(), "m1")
	assert.ErrorIs(t, err, wantErr)
}
