package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

func newNovelSvc(t *testing.T) (*service.NovelService, *mocks.MockJobRepository, *mocks.MockObjectStore) {
	t.Helper()
	jobs := mocks.NewMockJobRepository(t)
	store := mocks.NewMockObjectStore(t)
	return service.NewNovelService(jobs, store), jobs, store
}

func TestNovelService_Register_InlineText(t *testing.T) {
	svc, jobs, store := newNovelSvc(t)
	ctx := context.Background()

	// Inline text is written to R2, then the chapter's pages are replaced.
	store.EXPECT().Put(ctx, mock.Anything, []byte("hello"), mock.Anything).Return(nil)
	jobs.EXPECT().ReplacePages(ctx, "c1", mock.MatchedBy(func(p []domain.StoredPage) bool {
		return len(p) == 1 && p[0].Kind == domain.PageKindNovel
	})).Return(nil)

	page, err := svc.Register(ctx, domain.NovelRegisterRequest{ChapterID: "c1", Text: "hello"})
	require.NoError(t, err)
	assert.Equal(t, domain.PageKindNovel, page.Type)
	assert.Equal(t, "hello", page.Body)
}

func TestNovelService_Register_TextKey(t *testing.T) {
	svc, jobs, store := newNovelSvc(t)
	ctx := context.Background()

	store.EXPECT().Get(ctx, "novels/x.txt").Return([]byte("stored body"), "", nil)
	jobs.EXPECT().ReplacePages(ctx, "c1", mock.Anything).Return(nil)

	page, err := svc.Register(ctx, domain.NovelRegisterRequest{ChapterID: "c1", TextKey: "novels/x.txt"})
	require.NoError(t, err)
	assert.Equal(t, "stored body", page.Body)
}

func TestNovelService_Register_Validation(t *testing.T) {
	svc, _, _ := newNovelSvc(t)
	ctx := context.Background()

	_, err := svc.Register(ctx, domain.NovelRegisterRequest{Text: "x"}) // missing chapterId
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = svc.Register(ctx, domain.NovelRegisterRequest{ChapterID: "c1"}) // neither text nor key
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestNovelService_Register_StoreError(t *testing.T) {
	svc, _, store := newNovelSvc(t)
	ctx := context.Background()
	wantErr := errors.New("put failed")

	store.EXPECT().Put(ctx, mock.Anything, mock.Anything, mock.Anything).Return(wantErr)

	_, err := svc.Register(ctx, domain.NovelRegisterRequest{ChapterID: "c1", Text: "x"})
	assert.ErrorIs(t, err, wantErr)
}
