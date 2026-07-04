package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/convert"
	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
	"github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

func newConvertSvc(t *testing.T) (*service.ConvertService, *mocks.MockJobRepository, *mocks.MockObjectStore, *mocks.MockArchiveConverter) {
	t.Helper()
	jobs := mocks.NewMockJobRepository(t)
	store := mocks.NewMockObjectStore(t)
	conv := mocks.NewMockArchiveConverter(t)
	return service.NewConvertService(jobs, store, conv, time.Minute), jobs, store, conv
}

func TestConvertService_Convert_RejectsEmptySourceKey(t *testing.T) {
	svc, _, _, _ := newConvertSvc(t)

	_, err := svc.Convert(context.Background(), domain.ConvertRequest{})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestConvertService_Convert_CreateError(t *testing.T) {
	svc, jobs, _, _ := newConvertSvc(t)
	wantErr := errors.New("insert failed")

	jobs.EXPECT().Create(mock.Anything, mock.Anything).Return(wantErr)

	_, err := svc.Convert(context.Background(), domain.ConvertRequest{SourceKey: "u/a.cbz"})
	assert.ErrorIs(t, err, wantErr)
}

func TestConvertService_Convert_HappyPath_PersistsPagesForChapter(t *testing.T) {
	svc, jobs, store, conv := newConvertSvc(t)
	ctx := context.Background()
	req := domain.ConvertRequest{
		SourceKey:    "uploads/a.cbz",
		Format:       domain.FormatCBZ, // explicit hint skips magic detection
		OutputPrefix: "out/",
		ChapterID:    "ch1",
	}

	jobs.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertRunning, 0, "").Return(nil)
	store.EXPECT().Get(mock.Anything, "uploads/a.cbz").Return([]byte("PK-archive"), "application/zip", nil)
	conv.EXPECT().Convert(mock.Anything, domain.FormatCBZ, []byte("PK-archive")).
		Return([]convert.Result{{Index: 0, Data: []byte("avif"), Width: 800, Height: 1200}}, nil)
	store.EXPECT().Put(mock.Anything, "out/page-0000.avif", []byte("avif"), "image/avif").Return(nil)
	store.EXPECT().PublicURL("out/page-0000.avif").Return("")
	jobs.EXPECT().ReplacePages(mock.Anything, "ch1", []domain.StoredPage{
		{Index: 0, R2Key: "out/page-0000.avif", Width: 800, Height: 1200},
	}).Return(nil)
	jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertDone, 1, "").Return(nil)

	res, err := svc.Convert(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, domain.ConvertDone, res.Job.Status)
	assert.Equal(t, 1, res.Job.PageCount)
	require.Len(t, res.Pages, 1)
	assert.Equal(t, "/v1/image?key=out/page-0000.avif", res.Pages[0].ImageURL)
}

func TestConvertService_Convert_ConverterError_MarksFailed(t *testing.T) {
	svc, jobs, store, conv := newConvertSvc(t)
	ctx := context.Background()
	req := domain.ConvertRequest{SourceKey: "uploads/a.cbz", Format: domain.FormatCBZ, OutputPrefix: "out/"}
	convErr := errors.New("decode boom")

	jobs.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertRunning, 0, "").Return(nil)
	store.EXPECT().Get(mock.Anything, "uploads/a.cbz").Return([]byte("x"), "", nil)
	conv.EXPECT().Convert(mock.Anything, domain.FormatCBZ, []byte("x")).Return(nil, convErr)
	// Failure path records the error and a failed status.
	jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertFailed, 0, convErr.Error()).Return(nil)

	res, err := svc.Convert(ctx, req)
	assert.ErrorIs(t, err, convErr)
	assert.Equal(t, domain.ConvertFailed, res.Job.Status)
}

func TestConvertService_Convert_FetchSourceError(t *testing.T) {
	svc, jobs, store, _ := newConvertSvc(t)
	ctx := context.Background()
	req := domain.ConvertRequest{SourceKey: "uploads/a.cbz", Format: domain.FormatCBZ}
	getErr := errors.New("r2 down")

	jobs.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertRunning, 0, "").Return(nil)
	store.EXPECT().Get(mock.Anything, "uploads/a.cbz").Return(nil, "", getErr)
	jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertFailed, 0, mock.Anything).Return(nil)

	_, err := svc.Convert(ctx, req)
	assert.ErrorIs(t, err, getErr)
}

func TestConvertService_Job_Delegates(t *testing.T) {
	svc, jobs, _, _ := newConvertSvc(t)
	ctx := context.Background()
	want := domain.ConvertJob{ID: "j1", Status: domain.ConvertDone}

	jobs.EXPECT().Get(ctx, "j1").Return(want, nil)

	got, err := svc.Job(ctx, "j1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
