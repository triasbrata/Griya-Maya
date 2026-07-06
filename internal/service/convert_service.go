package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// ConvertService serves the browser-side ingest flow: it mints presigned R2 PUT
// URLs so the browser can upload AVIF pages directly, then registers those
// uploaded pages onto a chapter. Encoding itself happens in the browser.
type ConvertService struct {
	jobs  JobRepository
	store ObjectStore
}

// NewConvertService wires a ConvertService.
func NewConvertService(jobs JobRepository, store ObjectStore) *ConvertService {
	return &ConvertService{jobs: jobs, store: store}
}

// presignPutTTL bounds how long a minted direct-upload URL stays valid.
const presignPutTTL = 30 * time.Minute

// maxPresignBatch caps how many upload URLs a single presign request may mint.
const maxPresignBatch = 5000

// PresignItem is one page's target R2 key plus the presigned PUT URL the client
// uploads its AVIF bytes to.
type PresignItem struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// PresignResult is the batch of upload targets returned to the browser: one
// fresh prefix and `count` ordered items (page-0000.avif … page-NNNN.avif).
type PresignResult struct {
	Prefix string        `json:"prefix"`
	Items  []PresignItem `json:"items"`
}

// PresignUploads mints `count` presigned PUT URLs under one fresh prefix so the
// browser can encode AVIF pages and upload them straight to R2. contentType
// defaults to "image/avif" when empty.
func (s *ConvertService) PresignUploads(ctx context.Context, count int, contentType string) (PresignResult, error) {
	if count < 1 || count > maxPresignBatch {
		return PresignResult{}, fmt.Errorf("%w: count must be between 1 and %d", domain.ErrInvalidInput, maxPresignBatch)
	}
	if contentType == "" {
		contentType = "image/avif"
	}

	prefix := "pages/" + uuid.NewString() + "/"
	items := make([]PresignItem, 0, count)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%spage-%04d.avif", prefix, i)
		url, err := s.store.PresignPut(ctx, key, presignPutTTL, contentType)
		if err != nil {
			return PresignResult{}, fmt.Errorf("presign upload %d: %w", i, err)
		}
		items = append(items, PresignItem{Key: key, URL: url})
	}
	return PresignResult{Prefix: prefix, Items: items}, nil
}

// RegisterPages replaces a chapter's page rows with pages the browser already
// encoded and uploaded to R2 (via PresignUploads). It persists the stored pages
// and returns them as readable domain.Page with resolved image URLs.
func (s *ConvertService) RegisterPages(ctx context.Context, chapterID string, pages []domain.StoredPage) ([]domain.Page, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("%w: pages is required", domain.ErrInvalidInput)
	}
	for i, p := range pages {
		if strings.TrimSpace(p.R2Key) == "" {
			return nil, fmt.Errorf("%w: page %d is missing r2Key", domain.ErrInvalidInput, i)
		}
	}

	if err := s.jobs.ReplacePages(ctx, chapterID, pages); err != nil {
		return nil, fmt.Errorf("persist pages: %w", err)
	}

	out := make([]domain.Page, 0, len(pages))
	for _, p := range pages {
		out = append(out, domain.Page{
			Index:    p.Index,
			ImageURL: s.publicOrProxy(p.R2Key),
			Width:    p.Width,
			Height:   p.Height,
		})
	}
	return out, nil
}

func (s *ConvertService) publicOrProxy(key string) string {
	if u := s.store.PublicURL(key); u != "" {
		return u
	}
	// Relative proxy path; the MangaService uses absolute URLs for reads. Here
	// we keep it relative since the convert response is informational.
	return "/v1/image?key=" + key
}
