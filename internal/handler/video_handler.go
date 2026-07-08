package handler

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/service"
)

// VideoHandler exposes HLS upload/register endpoints plus a streaming proxy for
// playlists and segments stored in R2.
type VideoHandler struct {
	svc   VideoService
	store service.ObjectStore
}

// NewVideoHandler wires a VideoHandler.
func NewVideoHandler(svc VideoService, store service.ObjectStore) *VideoHandler {
	return &VideoHandler{svc: svc, store: store}
}

// VideoUploadResponse lists the R2 keys written for an uploaded HLS bundle.
type VideoUploadResponse struct {
	Prefix      string   `json:"prefix"`
	PlaylistKey string   `json:"playlistKey"`
	Keys        []string `json:"keys"`
}

// Presign godoc
// @Summary  Mint presigned R2 PUT URLs for an HLS bundle (direct browser→R2 upload).
// @Description Preferred over /v1/video/upload: the browser PUTs each bundle file straight to R2 (no container hop), then registers the returned playlistKey via POST /v1/video. Send every file's name (and optional contentType); keys keep the basenames under one prefix so relative segment URIs resolve.
// @Tags     video
// @Accept   json
// @Produce  json
// @Param    request body domain.VideoPresignRequest true "Bundle file names to presign"
// @Success  200 {object} service.VideoPresignResult
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/video/presign [post]
func (h *VideoHandler) Presign(ctx context.Context, c *app.RequestContext) {
	var req domain.VideoPresignRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := h.svc.PresignUploads(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, res)
}

// Upload godoc
// @Summary  Upload an HLS bundle (m3u8 + segments) into R2 (multipart field "files").
// @Description Store an already-segmented HLS bundle. Send the playlist and every segment as repeated "files" parts. Returns the written keys and the detected playlist key to pass to POST /v1/video.
// @Tags     video
// @Accept   multipart/form-data
// @Produce  json
// @Param    files formData file   true  "HLS files (repeat: index.m3u8 + segments)"
// @Param    prefix formData string false "Target R2 key prefix (defaults to hls/{uuid}/)"
// @Success  201 {object} handler.VideoUploadResponse
// @Security BearerAuth
// @Router   /v1/video/upload [post]
func (h *VideoHandler) Upload(ctx context.Context, c *app.RequestContext) {
	form, err := c.MultipartForm()
	if err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "multipart/form-data body is required")
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["file"]
	}
	if len(files) == 0 {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "at least one 'files' part is required")
		return
	}

	prefix := strings.TrimSpace(firstFormValue(form.Value["prefix"]))
	if prefix == "" {
		prefix = "hls/" + uuid.NewString() + "/"
	}
	prefix = strings.TrimLeft(prefix, "/")
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	keys := make([]string, 0, len(files))
	playlistKey := ""
	for _, fh := range files {
		name := filepath.Base(fh.Filename)
		if name == "" || name == "." || name == "/" {
			writeErr(c, consts.StatusBadRequest, "invalid_input", "a file part has an empty filename")
			return
		}
		f, err := fh.Open()
		if err != nil {
			writeError(c, err)
			return
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			writeError(c, err)
			return
		}

		key := prefix + name
		if err := h.store.Put(ctx, key, data, hlsContentType(name)); err != nil {
			writeError(c, err)
			return
		}
		keys = append(keys, key)

		if strings.EqualFold(filepath.Ext(name), ".m3u8") {
			// Prefer a master/index playlist; otherwise keep the first .m3u8 seen.
			if playlistKey == "" || strings.EqualFold(name, "index.m3u8") {
				playlistKey = key
			}
		}
	}

	if playlistKey == "" {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "no .m3u8 playlist found among the uploaded files")
		return
	}

	writeOK(c, consts.StatusCreated, VideoUploadResponse{Prefix: prefix, PlaylistKey: playlistKey, Keys: keys})
}

// Register godoc
// @Summary  Register an uploaded HLS playlist as a chapter's video page.
// @Tags     video
// @Accept   json
// @Produce  json
// @Param    request body domain.VideoRegisterRequest true "Video registration"
// @Success  200 {object} domain.Page
// @Failure  400 {object} handler.ErrorResponse
// @Security BearerAuth
// @Router   /v1/video [post]
func (h *VideoHandler) Register(ctx context.Context, c *app.RequestContext) {
	var req domain.VideoRegisterRequest
	if err := c.BindJSON(&req); err != nil {
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	page, err := h.svc.Register(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	writeOK(c, consts.StatusOK, page)
}

// Stream godoc
// @Summary  Proxy an HLS playlist/segment object from R2 (used when no public R2 domain).
// @Description Path-based so a playlist's relative segment URIs resolve. Supports HTTP Range for segment seeking.
// @Tags     video
// @Produce  application/vnd.apple.mpegurl
// @Param    key path string true "R2 object key (path)"
// @Success  200 {file} binary
// @Success  206 {file} binary
// @Router   /v1/stream/{key} [get]
func (h *VideoHandler) Stream(ctx context.Context, c *app.RequestContext) {
	key := strings.TrimLeft(c.Param("key"), "/")
	if key == "" {
		writeErr(c, consts.StatusBadRequest, "invalid_input", "key is required")
		return
	}

	data, contentType, err := h.store.Get(ctx, key)
	if err != nil {
		writeError(c, err)
		return
	}
	if ct := hlsContentType(key); ct != "application/octet-stream" {
		// Prefer the extension-derived type; stored uploads may have a generic CT.
		contentType = ct
	} else if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Playlists must not be cached long (they can be re-registered); segments are
	// immutable and safe to cache aggressively.
	if strings.EqualFold(filepath.Ext(key), ".m3u8") {
		c.Header("Cache-Control", "public, max-age=10")
	} else {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	}
	c.Header("Accept-Ranges", "bytes")

	// Honor a single-range request (AVPlayer seeks within segments this way).
	if start, end, ok := parseByteRange(string(c.GetHeader("Range")), len(data)); ok {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		c.Data(consts.StatusPartialContent, contentType, data[start:end+1])
		return
	}
	c.Data(consts.StatusOK, contentType, data)
}

// firstFormValue returns the first non-empty value of a multipart form field.
func firstFormValue(values []string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// hlsContentType maps a filename to its HLS MIME type, defaulting to
// application/octet-stream for unknown extensions. Thin alias over the domain
// helper so the upload/presign path and the stream proxy agree.
func hlsContentType(name string) string {
	return domain.HLSContentType(name)
}

// parseByteRange parses a single "bytes=start-end" header against a known size.
// It returns an inclusive [start, end] and ok=false when the header is absent,
// multi-range, or unsatisfiable (caller then serves the full body).
func parseByteRange(header string, size int) (int, int, bool) {
	const prefix = "bytes="
	if size == 0 || !strings.HasPrefix(header, prefix) {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(header, prefix)
	if strings.Contains(spec, ",") {
		return 0, 0, false // multi-range not supported; fall back to full body
	}
	before, after, found := strings.Cut(spec, "-")
	if !found {
		return 0, 0, false
	}
	startStr, endStr := strings.TrimSpace(before), strings.TrimSpace(after)

	// Suffix range: "bytes=-N" → last N bytes.
	if startStr == "" {
		n, err := strconv.Atoi(endStr)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	}

	start, err := strconv.Atoi(startStr)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	end := size - 1
	if endStr != "" {
		e, err := strconv.Atoi(endStr)
		if err != nil || e < start {
			return 0, 0, false
		}
		if e < end {
			end = e
		}
	}
	return start, end, true
}
