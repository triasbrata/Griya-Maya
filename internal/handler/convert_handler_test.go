package handler_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/handler"
	"github.com/triasbrata/mihon-manga-server/internal/handler/mocks"
	"github.com/triasbrata/mihon-manga-server/internal/service"
	svcmocks "github.com/triasbrata/mihon-manga-server/internal/service/mocks"
)

// multipartFile builds a multipart/form-data body with one file part plus extra
// text fields, returning the body and its content type.
func multipartFile(t *testing.T, field, filename string, data []byte, fields map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	require.NoError(t, w.Close())
	return buf.Bytes(), w.FormDataContentType()
}

func TestConvertHandler_Upload_StoresAndReturnsKey(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	store := svcmocks.NewMockObjectStore(t)
	h := handler.NewConvertHandler(svc, store)

	body, ct := multipartFile(t, "file", "book.cbz", []byte("PKarchive"), map[string]string{"key": "uploads/book.cbz"})
	store.EXPECT().Put(mock.Anything, "uploads/book.cbz", []byte("PKarchive"), "application/octet-stream").Return(nil)

	c := newCtx("POST", "/v1/convert/upload", nil, body, ct)
	h.Upload(context.Background(), c)

	assert.Equal(t, consts.StatusCreated, c.Response.StatusCode())
	var out handler.UploadResponse
	decodeJSON(t, c, &out)
	assert.Equal(t, "uploads/book.cbz", out.SourceKey)
}

func TestConvertHandler_Upload_MissingFile(t *testing.T) {
	h := handler.NewConvertHandler(mocks.NewMockConvertService(t), svcmocks.NewMockObjectStore(t))

	c := newCtx("POST", "/v1/convert/upload", nil, []byte("not-multipart"), "application/json")
	h.Upload(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConvertHandler_Convert_OK(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	h := handler.NewConvertHandler(svc, svcmocks.NewMockObjectStore(t))

	req := domain.ConvertRequest{SourceKey: "uploads/a.cbz", ChapterID: "ch1"}
	result := service.ConvertResult{
		Job:   domain.ConvertJob{ID: "j1", Status: domain.ConvertDone, PageCount: 2},
		Pages: []domain.Page{{Index: 0}, {Index: 1}},
	}
	svc.EXPECT().Convert(mock.Anything, req).Return(result, nil)

	c := newCtx("POST", "/v1/convert", nil, []byte(`{"sourceKey":"uploads/a.cbz","chapterId":"ch1"}`), "application/json")
	h.Convert(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var out service.ConvertResult
	decodeJSON(t, c, &out)
	assert.Equal(t, "j1", out.Job.ID)
	assert.Len(t, out.Pages, 2)
}

func TestConvertHandler_Convert_BadJSON(t *testing.T) {
	h := handler.NewConvertHandler(mocks.NewMockConvertService(t), svcmocks.NewMockObjectStore(t))

	c := newCtx("POST", "/v1/convert", nil, []byte(`{not json`), "application/json")
	h.Convert(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestConvertHandler_Convert_ServiceError(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	h := handler.NewConvertHandler(svc, svcmocks.NewMockObjectStore(t))

	svc.EXPECT().Convert(mock.Anything, mock.Anything).
		Return(service.ConvertResult{}, domain.ErrUnsupportedFormat)

	c := newCtx("POST", "/v1/convert", nil, []byte(`{"sourceKey":"x"}`), "application/json")
	h.Convert(context.Background(), c)
	assert.Equal(t, consts.StatusUnsupportedMediaType, c.Response.StatusCode())
}

func TestConvertHandler_JobStatus(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	h := handler.NewConvertHandler(svc, svcmocks.NewMockObjectStore(t))

	svc.EXPECT().Job(mock.Anything, "j1").
		Return(domain.ConvertJob{ID: "j1", Status: domain.ConvertRunning}, nil)

	c := newCtx("GET", "/v1/convert/jobs/j1", map[string]string{"id": "j1"}, nil, "")
	h.JobStatus(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var out domain.ConvertJob
	decodeJSON(t, c, &out)
	assert.Equal(t, domain.ConvertRunning, out.Status)
}

func TestConvertHandler_JobStatus_NotFound(t *testing.T) {
	svc := mocks.NewMockConvertService(t)
	h := handler.NewConvertHandler(svc, svcmocks.NewMockObjectStore(t))

	svc.EXPECT().Job(mock.Anything, "missing").Return(domain.ConvertJob{}, domain.ErrNotFound)

	c := newCtx("GET", "/v1/convert/jobs/missing", map[string]string{"id": "missing"}, nil, "")
	h.JobStatus(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())
}
