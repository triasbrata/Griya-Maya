//go:build !mupdf

package convert

import (
	"fmt"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// pdfExtractor is a no-op unless the binary is built with `-tags mupdf`, which
// links MuPDF (via go-fitz) for real PDF page rendering. Rendering vector PDFs
// requires native code, so it is opt-in and provided by the container image.
type pdfExtractor struct{}

func (pdfExtractor) Extract(_ []byte) ([]rawPage, error) {
	return nil, fmt.Errorf(
		"%w: PDF support requires building with -tags mupdf (MuPDF)",
		domain.ErrUnsupportedFormat,
	)
}
