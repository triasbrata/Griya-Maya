package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

func newSubTypeSvc(t *testing.T) (*service.SubTypeService, *mocks.MockMediaRepository) {
	t.Helper()
	repo := mocks.NewMockMediaRepository(t)
	return service.NewSubTypeService(repo), repo
}

func TestSubTypeService_List_FlattensAndSorts(t *testing.T) {
	svc, repo := newSubTypeSvc(t)
	ctx := context.Background()

	repo.EXPECT().SubTypeVocab(ctx).Return(map[domain.MediaType][]domain.SubType{
		domain.MediaNovel: {{Slug: "web_novel", Name: "Web Novel"}},
		domain.MediaManga: {{Slug: "manhwa", Name: "Manhwa"}, {Slug: "manga", Name: "Manga"}},
	}, nil)

	got, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 3)
	// Sorted by type (manga < novel) then slug (manga < manhwa).
	assert.Equal(t, domain.MediaManga, got[0].Type)
	assert.Equal(t, "manga", got[0].Slug)
	assert.Equal(t, "manhwa", got[1].Slug)
	assert.Equal(t, domain.MediaNovel, got[2].Type)
	assert.Equal(t, "web_novel", got[2].Slug)
}

func TestSubTypeService_Create_PersistsWhenNew(t *testing.T) {
	svc, repo := newSubTypeSvc(t)
	ctx := context.Background()

	repo.EXPECT().SubTypeVocab(ctx).Return(map[domain.MediaType][]domain.SubType{}, nil)
	repo.EXPECT().CreateSubType(ctx, mock.MatchedBy(func(st domain.SubType) bool {
		return st.Slug == "webtoon" && st.Type == domain.MediaManga && st.Name == "Webtoon"
	})).Return(nil)

	got, err := svc.Create(ctx, domain.SubTypeWriteRequest{Slug: "  webtoon  ", Type: domain.MediaManga, Name: " Webtoon "})
	require.NoError(t, err)
	assert.Equal(t, "webtoon", got.Slug)
	assert.Equal(t, domain.MediaManga, got.Type)
}

func TestSubTypeService_Create_RejectsDuplicate(t *testing.T) {
	svc, repo := newSubTypeSvc(t)
	ctx := context.Background()

	repo.EXPECT().SubTypeVocab(ctx).Return(map[domain.MediaType][]domain.SubType{
		domain.MediaManga: {{Slug: "manhwa", Name: "Manhwa"}},
	}, nil)

	_, err := svc.Create(ctx, domain.SubTypeWriteRequest{Slug: "manhwa", Type: domain.MediaManga, Name: "Manhwa"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestSubTypeService_Create_Validation(t *testing.T) {
	svc, _ := newSubTypeSvc(t)
	ctx := context.Background()

	cases := []domain.SubTypeWriteRequest{
		{Type: domain.MediaManga, Name: "X"},                       // missing slug
		{Slug: "x", Type: domain.MediaManga},                       // missing name
		{Slug: "x", Type: domain.MediaType("bogus"), Name: "X"},    // bad type
	}
	for _, req := range cases {
		_, err := svc.Create(ctx, req)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	}
}

func TestSubTypeService_Update(t *testing.T) {
	svc, repo := newSubTypeSvc(t)
	ctx := context.Background()

	repo.EXPECT().SubTypeVocab(ctx).Return(map[domain.MediaType][]domain.SubType{
		domain.MediaManga: {{Slug: "manhwa", Name: "Manhwa"}},
	}, nil)
	repo.EXPECT().UpdateSubType(ctx, "manhwa", mock.MatchedBy(func(st domain.SubType) bool {
		return st.Slug == "manhwa" && st.Name == "Manhwa (KR)"
	})).Return(nil)

	got, err := svc.Update(ctx, "manhwa", domain.SubTypeWriteRequest{Type: domain.MediaManga, Name: "Manhwa (KR)"})
	require.NoError(t, err)
	assert.Equal(t, "Manhwa (KR)", got.Name)
}

func TestSubTypeService_Update_NotFound(t *testing.T) {
	svc, repo := newSubTypeSvc(t)
	ctx := context.Background()

	repo.EXPECT().SubTypeVocab(ctx).Return(map[domain.MediaType][]domain.SubType{}, nil)

	_, err := svc.Update(ctx, "missing", domain.SubTypeWriteRequest{Type: domain.MediaManga, Name: "X"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSubTypeService_Update_Validation(t *testing.T) {
	svc, _ := newSubTypeSvc(t)
	ctx := context.Background()

	_, err := svc.Update(ctx, "  ", domain.SubTypeWriteRequest{Type: domain.MediaManga, Name: "X"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestSubTypeService_Delete(t *testing.T) {
	svc, repo := newSubTypeSvc(t)
	ctx := context.Background()

	repo.EXPECT().DeleteSubType(ctx, "manhwa").Return(nil)
	require.NoError(t, svc.Delete(ctx, "manhwa"))

	assert.ErrorIs(t, svc.Delete(ctx, "  "), domain.ErrInvalidInput)
}
