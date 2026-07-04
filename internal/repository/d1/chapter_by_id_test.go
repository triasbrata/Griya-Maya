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

func TestMediaRepo_ChapterByID(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	row := map[string]any{
		"id": "c1", "media_id": "m1", "url": "u", "name": "Ch 1",
		"number": float64(1), "scanlator": "", "date_upload": float64(0), "format": "",
	}
	q.EXPECT().Query(mock.Anything, sqlHasPrefix("SELECT"), "c1").
		Return([]map[string]any{row}, nil).Once()

	got, err := repo.ChapterByID(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "c1", got.ID)
	assert.Equal(t, "m1", got.MediaID)
}

func TestMediaRepo_ChapterByID_NotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	q.EXPECT().Query(mock.Anything, mock.Anything, "missing").Return(nil, nil).Once()

	_, err := repo.ChapterByID(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMediaRepo_SetMediaCover(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &MediaRepo{db: q}
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE media SET cover_url"),
		"m1", "covers/m1.avif", mock.Anything).Return(nil)

	require.NoError(t, repo.SetMediaCover(context.Background(), "m1", "covers/m1.avif"))
}
