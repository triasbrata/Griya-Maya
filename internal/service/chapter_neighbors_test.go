package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

func chapterList() []domain.Chapter {
	return []domain.Chapter{
		{ID: "c1", MediaID: "m1", Number: 1},
		{ID: "c2", MediaID: "m1", Number: 2},
		{ID: "c3", MediaID: "m1", Number: 3},
	}
}

func TestMediaService_ChapterNeighbors_Middle(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	chs := chapterList()
	repo.EXPECT().ChapterByID(mock.Anything, "c2").Return(chs[1], nil)
	repo.EXPECT().Chapters(mock.Anything, "m1").Return(chs, nil)

	got, err := svc.ChapterNeighbors(ctx, "c2")
	require.NoError(t, err)
	require.NotNil(t, got.Previous)
	require.NotNil(t, got.Next)
	assert.Equal(t, "c1", got.Previous.ID)
	assert.Equal(t, "c3", got.Next.ID)
}

func TestMediaService_ChapterNeighbors_First(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	chs := chapterList()
	repo.EXPECT().ChapterByID(mock.Anything, "c1").Return(chs[0], nil)
	repo.EXPECT().Chapters(mock.Anything, "m1").Return(chs, nil)

	got, err := svc.ChapterNeighbors(ctx, "c1")
	require.NoError(t, err)
	assert.Nil(t, got.Previous)
	require.NotNil(t, got.Next)
	assert.Equal(t, "c2", got.Next.ID)
}

func TestMediaService_ChapterNeighbors_Last(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	ctx := context.Background()
	chs := chapterList()
	repo.EXPECT().ChapterByID(mock.Anything, "c3").Return(chs[2], nil)
	repo.EXPECT().Chapters(mock.Anything, "m1").Return(chs, nil)

	got, err := svc.ChapterNeighbors(ctx, "c3")
	require.NoError(t, err)
	require.NotNil(t, got.Previous)
	assert.Equal(t, "c2", got.Previous.ID)
	assert.Nil(t, got.Next)
}

func TestMediaService_ChapterNeighbors_NotFound(t *testing.T) {
	svc, repo, _ := newMediaSvc(t, "")
	repo.EXPECT().ChapterByID(mock.Anything, "missing").Return(domain.Chapter{}, domain.ErrNotFound)

	_, err := svc.ChapterNeighbors(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
