package d1

import (
	"context"
	"errors"
	"strings"
	"testing"

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
