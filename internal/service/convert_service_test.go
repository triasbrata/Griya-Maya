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

func TestConvertService_Convert_Segments_SplitsPagesPerChapter(t *testing.T) {
	svc, jobs, store, conv := newConvertSvc(t)
	ctx := context.Background()
	req := domain.ConvertRequest{
		SourceKey:    "uploads/vol.cbz",
		Format:       domain.FormatCBZ,
		OutputPrefix: "out/",
		Segments: []domain.ConvertSegment{
			{ChapterID: "ch1", StartPage: 1, EndPage: 2},
			{ChapterID: "ch2", StartPage: 3, EndPage: 3},
		},
	}

	results := []convert.Result{
		{Index: 0, Data: []byte("a0"), Width: 10, Height: 20},
		{Index: 1, Data: []byte("a1"), Width: 11, Height: 21},
		{Index: 2, Data: []byte("a2"), Width: 12, Height: 22},
	}

	jobs.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertRunning, 0, "").Return(nil)
	store.EXPECT().Get(mock.Anything, "uploads/vol.cbz").Return([]byte("PK"), "application/zip", nil)
	conv.EXPECT().Convert(mock.Anything, domain.FormatCBZ, []byte("PK")).Return(results, nil)
	store.EXPECT().Put(mock.Anything, "out/page-0000.avif", []byte("a0"), "image/avif").Return(nil)
	store.EXPECT().Put(mock.Anything, "out/page-0001.avif", []byte("a1"), "image/avif").Return(nil)
	store.EXPECT().Put(mock.Anything, "out/page-0002.avif", []byte("a2"), "image/avif").Return(nil)
	store.EXPECT().PublicURL(mock.Anything).Return("").Times(3)

	// ch1 gets pages 1-2, re-indexed 0-based; ch2 gets page 3 re-indexed to 0.
	jobs.EXPECT().ReplacePages(mock.Anything, "ch1", []domain.StoredPage{
		{Index: 0, R2Key: "out/page-0000.avif", Width: 10, Height: 20},
		{Index: 1, R2Key: "out/page-0001.avif", Width: 11, Height: 21},
	}).Return(nil)
	jobs.EXPECT().ReplacePages(mock.Anything, "ch2", []domain.StoredPage{
		{Index: 0, R2Key: "out/page-0002.avif", Width: 12, Height: 22},
	}).Return(nil)
	jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertDone, 3, "").Return(nil)

	res, err := svc.Convert(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, domain.ConvertDone, res.Job.Status)
	assert.Equal(t, 3, res.Job.PageCount)
	require.Len(t, res.Pages, 3)
}

func TestConvertService_Convert_Segments_Invalid(t *testing.T) {
	results := []convert.Result{
		{Index: 0, Data: []byte("a0"), Width: 10, Height: 20},
		{Index: 1, Data: []byte("a1"), Width: 11, Height: 21},
	}

	cases := map[string]domain.ConvertSegment{
		"empty chapterId": {ChapterID: "", StartPage: 1, EndPage: 1},
		"start below one": {ChapterID: "ch1", StartPage: 0, EndPage: 1},
		"start above end": {ChapterID: "ch1", StartPage: 2, EndPage: 1},
		"end past total":  {ChapterID: "ch1", StartPage: 1, EndPage: 5},
	}

	for name, seg := range cases {
		t.Run(name, func(t *testing.T) {
			svc, jobs, store, conv := newConvertSvc(t)
			ctx := context.Background()
			req := domain.ConvertRequest{
				SourceKey:    "uploads/vol.cbz",
				Format:       domain.FormatCBZ,
				OutputPrefix: "out/",
				Segments:     []domain.ConvertSegment{seg},
			}

			jobs.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
			jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertRunning, 0, "").Return(nil)
			store.EXPECT().Get(mock.Anything, "uploads/vol.cbz").Return([]byte("PK"), "application/zip", nil)
			conv.EXPECT().Convert(mock.Anything, domain.FormatCBZ, []byte("PK")).Return(results, nil)
			store.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, "image/avif").Return(nil).Times(2)
			store.EXPECT().PublicURL(mock.Anything).Return("").Times(2)
			jobs.EXPECT().UpdateStatus(mock.Anything, mock.Anything, domain.ConvertFailed, mock.Anything, mock.Anything).Return(nil)

			_, err := svc.Convert(ctx, req)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
}

func TestConvertService_Probe(t *testing.T) {
	t.Run("returns page count", func(t *testing.T) {
		svc, _, store, conv := newConvertSvc(t)
		ctx := context.Background()

		store.EXPECT().Get(mock.Anything, "uploads/a.cbz").Return([]byte("PK"), "application/zip", nil)
		conv.EXPECT().PageCount(mock.Anything, domain.FormatCBZ, []byte("PK")).Return(7, nil)

		res, err := svc.Probe(ctx, domain.ConvertRequest{SourceKey: "uploads/a.cbz", Format: domain.FormatCBZ})
		require.NoError(t, err)
		assert.Equal(t, domain.FormatCBZ, res.Format)
		assert.Equal(t, 7, res.PageCount)
	})

	t.Run("rejects empty sourceKey", func(t *testing.T) {
		svc, _, _, _ := newConvertSvc(t)
		_, err := svc.Probe(context.Background(), domain.ConvertRequest{})
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})
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
