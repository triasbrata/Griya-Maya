package d1

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/repository/d1/mocks"
)

func sqlHasPrefix(prefix string) any {
	return mock.MatchedBy(func(s string) bool {
		return strings.HasPrefix(strings.TrimSpace(s), prefix)
	})
}

func TestJobRepo_Create(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &JobRepo{db: q}

	var params []any
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO convert_job"), anyN(11)...).
		RunAndReturn(func(_ context.Context, _ string, p ...any) error {
			params = p
			return nil
		})

	job := domain.ConvertJob{
		ID: "j1", SourceKey: "u/a.cbz", Format: domain.FormatCBZ,
		OutputPrefix: "out/", ChapterID: "ch1", Status: domain.ConvertPending,
		CreatedAt: time.Unix(1000, 0), UpdatedAt: time.Unix(1000, 0),
	}
	require.NoError(t, repo.Create(context.Background(), job))
	require.Len(t, params, 11)
	assert.Equal(t, "j1", params[0])
	assert.Equal(t, "cbz", params[2]) // Format stringified
}

func TestJobRepo_UpdateStatus(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &JobRepo{db: q}

	var params []any
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("UPDATE convert_job"), anyN(5)...).
		RunAndReturn(func(_ context.Context, _ string, p ...any) error {
			params = p
			return nil
		})

	err := repo.UpdateStatus(context.Background(), "j1", domain.ConvertDone, 5, "")
	require.NoError(t, err)
	assert.Equal(t, "j1", params[0])
	assert.Equal(t, "done", params[1])
	assert.Equal(t, 5, params[2])
}

func TestJobRepo_Get_FoundAndNotFound(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &JobRepo{db: q}

	q.EXPECT().Query(mock.Anything, mock.Anything, "j1").Return([]map[string]any{
		{
			"id": "j1", "source_key": "u/a.cbz", "format": "cbz", "output_prefix": "out/",
			"media_id": "m1", "chapter_id": "ch1", "status": "done", "page_count": float64(3),
			"error": "", "created_at": float64(1000), "updated_at": float64(2000),
		},
	}, nil).Once()

	got, err := repo.Get(context.Background(), "j1")
	require.NoError(t, err)
	assert.Equal(t, "j1", got.ID)
	assert.Equal(t, domain.ConvertDone, got.Status)
	assert.Equal(t, 3, got.PageCount)

	q.EXPECT().Query(mock.Anything, mock.Anything, "missing").Return(nil, nil).Once()
	_, err = repo.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestJobRepo_Get_Error(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &JobRepo{db: q}
	wantErr := errors.New("boom")

	q.EXPECT().Query(mock.Anything, mock.Anything, "j1").Return(nil, wantErr)

	_, err := repo.Get(context.Background(), "j1")
	assert.ErrorIs(t, err, wantErr)
}

func TestJobRepo_ReplacePages_DeletesThenInserts(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &JobRepo{db: q}

	// First a DELETE for the chapter, then one INSERT per page.
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM page"), "ch1").Return(nil).Once()
	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("INSERT INTO page"), anyN(6)...).Return(nil).Twice()

	pages := []domain.StoredPage{
		{Index: 0, R2Key: "pages/a.avif", Width: 800, Height: 1200},
		{Index: 1, R2Key: "pages/b.avif", Width: 800, Height: 1200},
	}
	require.NoError(t, repo.ReplacePages(context.Background(), "ch1", pages))
}

func TestJobRepo_ReplacePages_DeleteError(t *testing.T) {
	q := mocks.NewMockQuerier(t)
	repo := &JobRepo{db: q}
	wantErr := errors.New("delete failed")

	q.EXPECT().Exec(mock.Anything, sqlHasPrefix("DELETE FROM page"), "ch1").Return(wantErr).Once()

	err := repo.ReplacePages(context.Background(), "ch1", []domain.StoredPage{{Index: 0, R2Key: "x"}})
	assert.ErrorIs(t, err, wantErr)
}
