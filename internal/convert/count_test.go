package convert

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// zipWith builds an in-memory zip from name->content entries. A trailing "/" in
// a name creates a directory entry.
func zipWith(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestConverter_PageCount_CBZ(t *testing.T) {
	c := NewConverter(EncodeOptions{})

	archive := zipWith(t, map[string]string{
		"001.jpg":    "a",
		"002.png":    "b",
		"003.webp":   "c",
		"sub/":       "", // directory entry — ignored
		"notes.txt":  "not an image",
		"cover.avif": "d",
	})

	n, err := c.PageCount(context.Background(), domain.FormatCBZ, archive)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	// 4 image entries (jpg, png, webp, avif); dir + txt ignored.
	if n != 4 {
		t.Errorf("PageCount = %d, want 4", n)
	}
}

func TestConverter_PageCount_CBZ_NoImages(t *testing.T) {
	c := NewConverter(EncodeOptions{})
	archive := zipWith(t, map[string]string{"notes.txt": "x"})

	_, err := c.PageCount(context.Background(), domain.FormatCBZ, archive)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("PageCount err = %v, want ErrInvalidInput", err)
	}
}

// EPUB has no fast counter, so PageCount falls back to Extract (image-only zip
// resolved via the all-images fallback).
func TestConverter_PageCount_EPUB_FallsBackToExtract(t *testing.T) {
	c := NewConverter(EncodeOptions{})
	archive := zipWith(t, map[string]string{
		"OEBPS/img/001.jpg": "a",
		"OEBPS/img/002.jpg": "b",
	})

	n, err := c.PageCount(context.Background(), domain.FormatEPUB, archive)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	if n != 2 {
		t.Errorf("PageCount = %d, want 2", n)
	}
}

func TestConverter_PageCount_UnsupportedFormat(t *testing.T) {
	c := NewConverter(EncodeOptions{})
	_, err := c.PageCount(context.Background(), "bogus", nil)
	if !errors.Is(err, domain.ErrUnsupportedFormat) {
		t.Errorf("PageCount err = %v, want ErrUnsupportedFormat", err)
	}
}
