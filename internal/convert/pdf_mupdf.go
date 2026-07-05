//go:build mupdf

package convert

import (
	"bytes"
	"fmt"
	"image/png"

	"github.com/gen2brain/go-fitz"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// pdfExtractor renders each PDF page to a raster image using MuPDF.
// Built only with `-tags mupdf`; the container image installs the toolchain.
type pdfExtractor struct{}

// renderDPI controls rasterization quality of vector PDF pages.
const renderDPI = 200

func (pdfExtractor) Extract(archive []byte) ([]rawPage, error) {
	doc, err := fitz.NewFromMemory(archive)
	if err != nil {
		return nil, fmt.Errorf("%w: open pdf: %v", domain.ErrInvalidInput, err)
	}
	defer doc.Close()

	n := doc.NumPage()
	if n == 0 {
		return nil, fmt.Errorf("%w: pdf has no pages", domain.ErrInvalidInput)
	}

	pages := make([]rawPage, 0, n)
	for i := 0; i < n; i++ {
		img, err := doc.ImageDPI(i, renderDPI)
		if err != nil {
			return nil, fmt.Errorf("render pdf page %d: %w", i, err)
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("encode pdf page %d: %w", i, err)
		}
		pages = append(pages, rawPage{
			name: fmt.Sprintf("%04d.png", i),
			data: buf.Bytes(),
		})
	}
	return pages, nil
}

// PageCount returns the PDF page count without rendering any page.
func (pdfExtractor) PageCount(archive []byte) (int, error) {
	doc, err := fitz.NewFromMemory(archive)
	if err != nil {
		return 0, fmt.Errorf("%w: open pdf: %v", domain.ErrInvalidInput, err)
	}
	defer doc.Close()

	n := doc.NumPage()
	if n == 0 {
		return 0, fmt.Errorf("%w: pdf has no pages", domain.ErrInvalidInput)
	}
	return n, nil
}
