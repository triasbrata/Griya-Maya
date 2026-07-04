package convert

import (
	"errors"
	"testing"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

func TestDetectFormat(t *testing.T) {
	pdfMagic := []byte("%PDF-1.7 ...")
	zipMagic := []byte("PK\x03\x04rest")

	cases := []struct {
		name    string
		hint    domain.ArchiveFormat
		key     string
		data    []byte
		want    domain.ArchiveFormat
		wantErr bool
	}{
		{"explicit hint wins", domain.FormatEPUB, "whatever.pdf", pdfMagic, domain.FormatEPUB, false},
		{"cbz extension", "", "book.cbz", nil, domain.FormatCBZ, false},
		{"epub extension", "", "book.EPUB", nil, domain.FormatEPUB, false},
		{"pdf extension", "", "doc.pdf", nil, domain.FormatPDF, false},
		{"pdf magic", "", "noext", pdfMagic, domain.FormatPDF, false},
		{"zip magic -> cbz", "", "noext", zipMagic, domain.FormatCBZ, false},
		{"undetectable", "", "mystery", []byte("xx"), "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectFormat(tc.hint, tc.key, tc.data)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrUnsupportedFormat) {
					t.Fatalf("want ErrUnsupportedFormat, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("DetectFormat = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractorFor(t *testing.T) {
	for _, f := range []domain.ArchiveFormat{domain.FormatCBZ, domain.FormatEPUB, domain.FormatPDF} {
		if _, err := extractorFor(f); err != nil {
			t.Errorf("extractorFor(%q) unexpected error: %v", f, err)
		}
	}
	if _, err := extractorFor("bogus"); !errors.Is(err, domain.ErrUnsupportedFormat) {
		t.Errorf("extractorFor(bogus) = %v, want ErrUnsupportedFormat", err)
	}
}
