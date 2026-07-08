package handler

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/stretchr/testify/assert"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

func TestParseByteRange(t *testing.T) {
	cases := []struct {
		name               string
		header             string
		size               int
		wantStart, wantEnd int
		wantOK             bool
	}{
		{"absent", "", 100, 0, 0, false},
		{"empty size", "bytes=0-10", 0, 0, 0, false},
		{"closed range", "bytes=2-5", 10, 2, 5, true},
		{"open-ended", "bytes=3-", 10, 3, 9, true},
		{"suffix", "bytes=-4", 10, 6, 9, true},
		{"suffix bigger than size", "bytes=-40", 10, 0, 9, true},
		{"end clamped to size", "bytes=0-999", 10, 0, 9, true},
		{"multi-range unsupported", "bytes=0-1,3-4", 10, 0, 0, false},
		{"start out of range", "bytes=20-25", 10, 0, 0, false},
		{"end before start", "bytes=5-2", 10, 0, 0, false},
		{"not a byte range", "items=0-1", 10, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, ok := parseByteRange(tc.header, tc.size)
			assert.Equal(t, tc.wantOK, ok)
			if ok {
				assert.Equal(t, tc.wantStart, start)
				assert.Equal(t, tc.wantEnd, end)
			}
		})
	}
}

func TestHLSContentType(t *testing.T) {
	cases := map[string]string{
		"index.m3u8":  "application/vnd.apple.mpegurl",
		"seg0.ts":     "video/mp2t",
		"seg0.m4s":    "video/mp4",
		"audio.aac":   "audio/aac",
		"subs.vtt":    "text/vtt",
		"unknown.bin": "application/octet-stream",
	}
	for name, want := range cases {
		assert.Equal(t, want, hlsContentType(name), name)
	}
}

func TestFirstFormValue(t *testing.T) {
	assert.Equal(t, "b", firstFormValue([]string{"", "  ", "b", "c"}))
	assert.Equal(t, "", firstFormValue([]string{"", "  "}))
	assert.Equal(t, "", firstFormValue(nil))
}

func TestQueryInt(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/x?page=7&bad=nope")
	assert.Equal(t, 7, queryInt(c, "page", 1))
	assert.Equal(t, 1, queryInt(c, "bad", 1))     // non-numeric -> default
	assert.Equal(t, 3, queryInt(c, "missing", 3)) // absent -> default
}

func TestQueryAll_RepeatedAndCommaJoined(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/x?genre=action&genre=comedy,drama&genre=+++")
	got := queryAll(c, "genre")
	assert.Equal(t, []string{"action", "comedy", "drama"}, got)
}

func TestParseCatalogFilter(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/x?sort=title&order=asc&type=manga&subType=manhwa")
	f := parseCatalogFilter(c)
	assert.Equal(t, "title", f.Sort)
	assert.True(t, f.Ascending)
	assert.Equal(t, []string{"manga"}, f.Types)
	assert.Equal(t, []string{"manhwa"}, f.SubTypes)
}

func TestParseCatalogFilter_Defaults(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/x")
	f := parseCatalogFilter(c)
	assert.False(t, f.Ascending)
	assert.Empty(t, f.Types)
	assert.Empty(t, f.SubTypes)
}

func TestWriteError_StatusMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{domain.ErrNotFound, consts.StatusNotFound},
		{domain.ErrInvalidInput, consts.StatusBadRequest},
		{domain.ErrUnsupportedFormat, consts.StatusUnsupportedMediaType},
		{domain.ErrUnauthorized, consts.StatusUnauthorized},
		{assertAnError{}, consts.StatusInternalServerError},
	}
	for _, tc := range cases {
		c := app.NewContext(0)
		writeError(c, tc.err)
		assert.Equal(t, tc.want, c.Response.StatusCode())
	}
}

type assertAnError struct{}

func (assertAnError) Error() string { return "boom" }

func TestWritePagination(t *testing.T) {
	hdr := func(c *app.RequestContext, k string) string { return string(c.Response.Header.Get(k)) }

	t.Run("offset omits unknown total", func(t *testing.T) {
		c := app.NewContext(0)
		writePagination(c, domain.OffsetPagination(2, 30, -1, true))
		assert.Equal(t, "offset", hdr(c, HdrPaginationKind))
		assert.Equal(t, "true", hdr(c, HdrPaginationHasNext))
		assert.Equal(t, "2", hdr(c, HdrPaginationPage))
		assert.Equal(t, "30", hdr(c, HdrPaginationPerPage))
		assert.Empty(t, hdr(c, HdrPaginationTotal))
		assert.Empty(t, hdr(c, HdrPaginationNextCursor))
	})

	t.Run("offset with total", func(t *testing.T) {
		c := app.NewContext(0)
		writePagination(c, domain.OffsetPagination(1, 30, 57, false))
		assert.Equal(t, "57", hdr(c, HdrPaginationTotal))
		assert.Equal(t, "false", hdr(c, HdrPaginationHasNext))
	})

	t.Run("cursor omits empty prev and offset fields", func(t *testing.T) {
		c := app.NewContext(0)
		writePagination(c, domain.CursorPagination(20, "next123", "", true))
		assert.Equal(t, "cursor", hdr(c, HdrPaginationKind))
		assert.Equal(t, "20", hdr(c, HdrPaginationPerPage))
		assert.Equal(t, "next123", hdr(c, HdrPaginationNextCursor))
		assert.Empty(t, hdr(c, HdrPaginationPrevCursor))
		assert.Empty(t, hdr(c, HdrPaginationPage))
		assert.Empty(t, hdr(c, HdrPaginationTotal))
	})
}

func TestQueryCursor(t *testing.T) {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/v1/x?cursor=abc123")
	assert.Equal(t, "abc123", queryCursor(c))

	empty := app.NewContext(0)
	empty.Request.SetRequestURI("/v1/x")
	assert.Empty(t, queryCursor(empty))
}
