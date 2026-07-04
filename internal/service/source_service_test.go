package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

func newSourceSvc(t *testing.T) (*SourceService, *mocks.MockSourceRepository) {
	t.Helper()
	repo := mocks.NewMockSourceRepository(t)
	return NewSourceService(repo), repo
}

func TestSourceService_List(t *testing.T) {
	svc, repo := newSourceSvc(t)
	repo.EXPECT().List(mock.Anything, true).Return([]domain.Source{{ID: "griyamedia"}}, nil)
	got, err := svc.List(context.Background(), true)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestSourceService_Create_DefaultsAndPersists(t *testing.T) {
	svc, repo := newSourceSvc(t)
	repo.EXPECT().Exists(mock.Anything, "s1").Return(false, nil)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(s domain.Source) bool {
		return s.ID == "s1" && s.Name == "S1" && s.Lang == "en" && s.Enabled
	})).Return(nil)
	repo.EXPECT().Get(mock.Anything, "s1").Return(domain.Source{ID: "s1", Name: "S1", Lang: "en", Enabled: true}, nil)

	got, err := svc.Create(context.Background(), domain.SourceWriteRequest{ID: "s1", Name: "S1"})
	require.NoError(t, err)
	assert.Equal(t, "s1", got.ID)
	assert.True(t, got.Enabled)
}

func TestSourceService_Create_DisabledFlagHonored(t *testing.T) {
	svc, repo := newSourceSvc(t)
	f := false
	repo.EXPECT().Exists(mock.Anything, "s1").Return(false, nil)
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(s domain.Source) bool { return !s.Enabled })).Return(nil)
	repo.EXPECT().Get(mock.Anything, "s1").Return(domain.Source{ID: "s1"}, nil)
	_, err := svc.Create(context.Background(), domain.SourceWriteRequest{ID: "s1", Name: "S1", Enabled: &f})
	require.NoError(t, err)
}

func TestSourceService_Create_Validation(t *testing.T) {
	svc, repo := newSourceSvc(t)
	_, err := svc.Create(context.Background(), domain.SourceWriteRequest{Name: "x"}) // missing id
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
	_, err = svc.Create(context.Background(), domain.SourceWriteRequest{ID: "s1"}) // missing name
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	repo.EXPECT().Exists(mock.Anything, "dup").Return(true, nil)
	_, err = svc.Create(context.Background(), domain.SourceWriteRequest{ID: "dup", Name: "D"}) // duplicate
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestSourceService_Update(t *testing.T) {
	svc, repo := newSourceSvc(t)
	repo.EXPECT().Get(mock.Anything, "s1").Return(domain.Source{ID: "s1"}, nil).Once()
	repo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(s domain.Source) bool {
		return s.ID == "s1" && s.Name == "New"
	})).Return(nil)
	repo.EXPECT().Get(mock.Anything, "s1").Return(domain.Source{ID: "s1", Name: "New"}, nil).Once()

	got, err := svc.Update(context.Background(), "s1", domain.SourceWriteRequest{Name: "New"})
	require.NoError(t, err)
	assert.Equal(t, "New", got.Name)
}

func TestSourceService_Update_NotFound(t *testing.T) {
	svc, repo := newSourceSvc(t)
	repo.EXPECT().Get(mock.Anything, "missing").Return(domain.Source{}, domain.ErrNotFound)
	_, err := svc.Update(context.Background(), "missing", domain.SourceWriteRequest{Name: "x"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSourceService_Delete_RefusesNonEmpty(t *testing.T) {
	svc, repo := newSourceSvc(t)
	repo.EXPECT().MediaCount(mock.Anything, "s1").Return(3, nil)
	assert.ErrorIs(t, svc.Delete(context.Background(), "s1"), domain.ErrInvalidInput)
}

func TestSourceService_Delete_OK(t *testing.T) {
	svc, repo := newSourceSvc(t)
	repo.EXPECT().MediaCount(mock.Anything, "s1").Return(0, nil)
	repo.EXPECT().Delete(mock.Anything, "s1").Return(nil)
	require.NoError(t, svc.Delete(context.Background(), "s1"))
}

func TestSourceService_Get_Validation(t *testing.T) {
	svc, _ := newSourceSvc(t)
	_, err := svc.Get(context.Background(), "  ")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}
