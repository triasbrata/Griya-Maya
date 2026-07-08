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

// multipartFiles builds a multipart body with several parts under one field.
func multipartFiles(t *testing.T, field string, files map[string][]byte, values map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for name, data := range files {
		fw, err := w.CreateFormFile(field, name)
		require.NoError(t, err)
		_, err = fw.Write(data)
		require.NoError(t, err)
	}
	for k, v := range values {
		require.NoError(t, w.WriteField(k, v))
	}
	require.NoError(t, w.Close())
	return buf.Bytes(), w.FormDataContentType()
}

func TestVideoHandler_Upload_StoresBundleAndDetectsPlaylist(t *testing.T) {
	svc := mocks.NewMockVideoService(t)
	store := svcmocks.NewMockObjectStore(t)
	h := handler.NewVideoHandler(svc, store)

	body, ct := multipartFiles(t, "files", map[string][]byte{
		"index.m3u8": []byte("#EXTM3U"),
		"seg0.ts":    []byte("tsdata"),
	}, map[string]string{"prefix": "hls/vid1"})

	store.EXPECT().Put(mock.Anything, "hls/vid1/index.m3u8", []byte("#EXTM3U"), "application/vnd.apple.mpegurl").Return(nil)
	store.EXPECT().Put(mock.Anything, "hls/vid1/seg0.ts", []byte("tsdata"), "video/mp2t").Return(nil)

	c := newCtx("POST", "/v1/video/upload", nil, body, ct)
	h.Upload(context.Background(), c)

	assert.Equal(t, consts.StatusCreated, c.Response.StatusCode())
	var out handler.VideoUploadResponse
	decodeJSON(t, c, &out)
	assert.Equal(t, "hls/vid1/index.m3u8", out.PlaylistKey)
	assert.Len(t, out.Keys, 2)
}

func TestVideoHandler_Upload_NoPlaylist(t *testing.T) {
	svc := mocks.NewMockVideoService(t)
	store := svcmocks.NewMockObjectStore(t)
	h := handler.NewVideoHandler(svc, store)

	body, ct := multipartFiles(t, "files", map[string][]byte{"seg0.ts": []byte("tsdata")}, nil)
	// The single segment is stored, then the missing-playlist check fails.
	store.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	c := newCtx("POST", "/v1/video/upload", nil, body, ct)
	h.Upload(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestVideoHandler_Upload_NotMultipart(t *testing.T) {
	h := handler.NewVideoHandler(mocks.NewMockVideoService(t), svcmocks.NewMockObjectStore(t))

	c := newCtx("POST", "/v1/video/upload", nil, []byte("{}"), "application/json")
	h.Upload(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestVideoHandler_Register_OK(t *testing.T) {
	svc := mocks.NewMockVideoService(t)
	h := handler.NewVideoHandler(svc, svcmocks.NewMockObjectStore(t))

	req := domain.VideoRegisterRequest{ChapterID: "ch1", PlaylistKey: "hls/a/index.m3u8"}
	svc.EXPECT().Register(mock.Anything, req).
		Return(domain.Page{Index: 0, Type: domain.PageKindVideo, ImageURL: "https://api/v1/stream/hls/a/index.m3u8"}, nil)

	c := newCtx("POST", "/v1/video", nil, []byte(`{"chapterId":"ch1","playlistKey":"hls/a/index.m3u8"}`), "application/json")
	h.Register(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var out domain.Page
	decodeJSON(t, c, &out)
	assert.Equal(t, domain.PageKindVideo, out.Type)
}

func TestVideoHandler_Register_ValidationError(t *testing.T) {
	svc := mocks.NewMockVideoService(t)
	h := handler.NewVideoHandler(svc, svcmocks.NewMockObjectStore(t))

	svc.EXPECT().Register(mock.Anything, mock.Anything).Return(domain.Page{}, domain.ErrInvalidInput)

	c := newCtx("POST", "/v1/video", nil, []byte(`{"chapterId":"ch1"}`), "application/json")
	h.Register(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestVideoHandler_Presign_OK(t *testing.T) {
	svc := mocks.NewMockVideoService(t)
	h := handler.NewVideoHandler(svc, svcmocks.NewMockObjectStore(t))

	req := domain.VideoPresignRequest{Files: []domain.VideoPresignFile{
		{Name: "index.m3u8"}, {Name: "v720_0.m4s"},
	}}
	svc.EXPECT().PresignUploads(mock.Anything, req).Return(service.VideoPresignResult{
		Prefix:      "hls/vid1/",
		PlaylistKey: "hls/vid1/index.m3u8",
		Items: []service.VideoPresignItem{
			{Name: "index.m3u8", Key: "hls/vid1/index.m3u8", URL: "https://r2/put/1", ContentType: "application/vnd.apple.mpegurl"},
			{Name: "v720_0.m4s", Key: "hls/vid1/v720_0.m4s", URL: "https://r2/put/2", ContentType: "video/mp4"},
		},
	}, nil)

	c := newCtx("POST", "/v1/video/presign", nil,
		[]byte(`{"files":[{"name":"index.m3u8"},{"name":"v720_0.m4s"}]}`), "application/json")
	h.Presign(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	var out service.VideoPresignResult
	decodeJSON(t, c, &out)
	assert.Equal(t, "hls/vid1/index.m3u8", out.PlaylistKey)
	assert.Len(t, out.Items, 2)
}

func TestVideoHandler_Presign_BadJSON(t *testing.T) {
	h := handler.NewVideoHandler(mocks.NewMockVideoService(t), svcmocks.NewMockObjectStore(t))

	c := newCtx("POST", "/v1/video/presign", nil, []byte(`not json`), "application/json")
	h.Presign(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestVideoHandler_Presign_ServiceError(t *testing.T) {
	svc := mocks.NewMockVideoService(t)
	h := handler.NewVideoHandler(svc, svcmocks.NewMockObjectStore(t))

	svc.EXPECT().PresignUploads(mock.Anything, mock.Anything).
		Return(service.VideoPresignResult{}, domain.ErrInvalidInput)

	c := newCtx("POST", "/v1/video/presign", nil, []byte(`{"files":[]}`), "application/json")
	h.Presign(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestVideoHandler_Stream_MissingKey(t *testing.T) {
	h := handler.NewVideoHandler(mocks.NewMockVideoService(t), svcmocks.NewMockObjectStore(t))

	c := newCtx("GET", "/v1/stream/", map[string]string{"key": ""}, nil, "")
	h.Stream(context.Background(), c)
	assert.Equal(t, consts.StatusBadRequest, c.Response.StatusCode())
}

func TestVideoHandler_Stream_FullBody(t *testing.T) {
	store := svcmocks.NewMockObjectStore(t)
	h := handler.NewVideoHandler(mocks.NewMockVideoService(t), store)

	store.EXPECT().Get(mock.Anything, "hls/a/index.m3u8").Return([]byte("#EXTM3U"), "", nil)

	c := newCtx("GET", "/v1/stream/hls/a/index.m3u8", map[string]string{"key": "hls/a/index.m3u8"}, nil, "")
	h.Stream(context.Background(), c)

	assert.Equal(t, consts.StatusOK, c.Response.StatusCode())
	assert.Equal(t, []byte("#EXTM3U"), c.Response.Body())
	assert.Contains(t, string(c.Response.Header.ContentType()), "mpegurl")
}

func TestVideoHandler_Stream_RangeRequest(t *testing.T) {
	store := svcmocks.NewMockObjectStore(t)
	h := handler.NewVideoHandler(mocks.NewMockVideoService(t), store)

	store.EXPECT().Get(mock.Anything, "hls/a/seg0.ts").Return([]byte("0123456789"), "", nil)

	c := newCtx("GET", "/v1/stream/hls/a/seg0.ts", map[string]string{"key": "hls/a/seg0.ts"}, nil, "")
	c.Request.Header.Set("Range", "bytes=2-5")
	h.Stream(context.Background(), c)

	assert.Equal(t, consts.StatusPartialContent, c.Response.StatusCode())
	assert.Equal(t, []byte("2345"), c.Response.Body())
	assert.Equal(t, "bytes 2-5/10", string(c.Response.Header.Peek("Content-Range")))
}

func TestVideoHandler_Stream_StoreError(t *testing.T) {
	store := svcmocks.NewMockObjectStore(t)
	h := handler.NewVideoHandler(mocks.NewMockVideoService(t), store)

	store.EXPECT().Get(mock.Anything, "missing").Return(nil, "", domain.ErrNotFound)

	c := newCtx("GET", "/v1/stream/missing", map[string]string{"key": "missing"}, nil, "")
	h.Stream(context.Background(), c)
	assert.Equal(t, consts.StatusNotFound, c.Response.StatusCode())
}
