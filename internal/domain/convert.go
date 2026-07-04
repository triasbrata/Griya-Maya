package domain

import (
	"errors"
	"time"
)

// ArchiveFormat is a supported input container.
type ArchiveFormat string

const (
	FormatCBZ  ArchiveFormat = "cbz"
	FormatEPUB ArchiveFormat = "epub"
	FormatPDF  ArchiveFormat = "pdf"
)

// ConvertStatus tracks a conversion job.
type ConvertStatus string

const (
	ConvertPending ConvertStatus = "pending"
	ConvertRunning ConvertStatus = "running"
	ConvertDone    ConvertStatus = "done"
	ConvertFailed  ConvertStatus = "failed"
)

// ConvertRequest asks the service to turn an archive already stored in R2 into
// a set of AVIF page objects, also stored in R2.
type ConvertRequest struct {
	// SourceKey is the R2 object key of the CBZ/EPUB/PDF to convert.
	SourceKey string `json:"sourceKey"`
	// Format overrides format detection when set.
	Format ArchiveFormat `json:"format,omitempty"`
	// OutputPrefix is the R2 key prefix under which page-NNNN.avif are written.
	// Defaults to "pages/<jobID>/".
	OutputPrefix string `json:"outputPrefix,omitempty"`
	// MediaID / ChapterID associate the produced pages with catalog rows.
	MediaID   string `json:"mediaId,omitempty"`
	ChapterID string `json:"chapterId,omitempty"`
}

// ConvertJob is the persisted state of a conversion.
type ConvertJob struct {
	ID           string        `json:"id"`
	SourceKey    string        `json:"sourceKey"`
	Format       ArchiveFormat `json:"format"`
	OutputPrefix string        `json:"outputPrefix"`
	MediaID      string        `json:"mediaId,omitempty"`
	ChapterID    string        `json:"chapterId,omitempty"`
	Status       ConvertStatus `json:"status"`
	PageCount    int           `json:"pageCount"`
	Error        string        `json:"error,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// Common typed errors surfaced to handlers for status mapping.
var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUnsupportedFormat = errors.New("unsupported archive format")
	ErrUnauthorized      = errors.New("unauthorized")
)
