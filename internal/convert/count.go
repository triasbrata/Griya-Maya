package convert

import (
	"context"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// pageCounter is an optional fast path some extractors implement to count
// pages without reading/decoding every image.
type pageCounter interface {
	PageCount(archive []byte) (int, error)
}

// PageCount returns the number of ordered pages in the archive without
// encoding them. Uses an extractor's fast path when available, else extracts.
func (c *Converter) PageCount(_ context.Context, format domain.ArchiveFormat, archive []byte) (int, error) {
	ext, err := extractorFor(format)
	if err != nil {
		return 0, err
	}
	if pc, ok := ext.(pageCounter); ok {
		return pc.PageCount(archive)
	}
	raw, err := ext.Extract(archive)
	if err != nil {
		return 0, err
	}
	return len(raw), nil
}
