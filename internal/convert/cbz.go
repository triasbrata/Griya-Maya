package convert

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// cbzExtractor reads a CBZ (ZIP of images).
type cbzExtractor struct{}

func (cbzExtractor) Extract(archive []byte) ([]rawPage, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("%w: open cbz: %v", domain.ErrInvalidInput, err)
	}

	var pages []rawPage
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isImageName(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", f.Name, err)
		}
		pages = append(pages, rawPage{name: f.Name, data: data})
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("%w: cbz has no images", domain.ErrInvalidInput)
	}
	sortPagesNaturally(pages)
	return pages, nil
}
