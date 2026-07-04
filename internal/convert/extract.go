package convert

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// rawPage is a single extracted image before AVIF encoding.
type rawPage struct {
	name string
	data []byte
}

// Extractor pulls ordered raster pages out of an archive's bytes.
type Extractor interface {
	Extract(archive []byte) ([]rawPage, error)
}

// DetectFormat resolves the archive format from an explicit hint, the object
// key extension, or magic bytes.
func DetectFormat(hint domain.ArchiveFormat, key string, data []byte) (domain.ArchiveFormat, error) {
	if hint != "" {
		return hint, nil
	}
	switch strings.ToLower(filepath.Ext(key)) {
	case ".cbz":
		return domain.FormatCBZ, nil
	case ".epub":
		return domain.FormatEPUB, nil
	case ".pdf":
		return domain.FormatPDF, nil
	}
	// Magic-byte fallback.
	switch {
	case len(data) >= 4 && string(data[:4]) == "%PDF":
		return domain.FormatPDF, nil
	case len(data) >= 2 && data[0] == 'P' && data[1] == 'K':
		// ZIP container — CBZ vs EPUB. EPUBs contain a mimetype entry, but we
		// default to CBZ; the EPUB path also handles image-only zips.
		return domain.FormatCBZ, nil
	}
	return "", fmt.Errorf("%w: cannot detect format for %q", domain.ErrUnsupportedFormat, key)
}

// extractorFor returns the extractor implementing a format.
func extractorFor(format domain.ArchiveFormat) (Extractor, error) {
	switch format {
	case domain.FormatCBZ:
		return cbzExtractor{}, nil
	case domain.FormatEPUB:
		return epubExtractor{}, nil
	case domain.FormatPDF:
		return pdfExtractor{}, nil // stub unless built with -tags mupdf
	default:
		return nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedFormat, format)
	}
}

var imageExtRE = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|webp|avif|bmp)$`)

func isImageName(name string) bool {
	return imageExtRE.MatchString(name)
}

// sortPagesNaturally orders entries the way a reader expects: numeric segments
// compared as numbers ("2" < "10"), everything else lexicographically.
func sortPagesNaturally(pages []rawPage) {
	sort.SliceStable(pages, func(i, j int) bool {
		return naturalLess(pages[i].name, pages[j].name)
	})
}

var digitsRE = regexp.MustCompile(`\d+|\D+`)

func naturalLess(a, b string) bool {
	as := digitsRE.FindAllString(strings.ToLower(a), -1)
	bs := digitsRE.FindAllString(strings.ToLower(b), -1)
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		if aerr == nil && berr == nil {
			return an < bn
		}
		return as[i] < bs[i]
	}
	return len(as) < len(bs)
}
