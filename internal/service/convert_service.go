package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/convert"
	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// ConvertService orchestrates archive -> AVIF conversion: pull from R2, convert,
// push AVIF pages back to R2, and record the job + pages in D1.
type ConvertService struct {
	jobs      JobRepository
	store     ObjectStore
	converter ArchiveConverter
	timeout   time.Duration
}

// NewConvertService wires a ConvertService.
func NewConvertService(jobs JobRepository, store ObjectStore, converter ArchiveConverter, timeout time.Duration) *ConvertService {
	return &ConvertService{jobs: jobs, store: store, converter: converter, timeout: timeout}
}

// ConvertResult is returned once conversion completes.
type ConvertResult struct {
	Job   domain.ConvertJob `json:"job"`
	Pages []domain.Page     `json:"pages"`
}

// Convert runs the full pipeline synchronously and returns the finished job.
// A container is long-lived, so synchronous processing (bounded by timeout) is
// the simplest correct model; swap in a queue consumer for very large batches.
func (s *ConvertService) Convert(ctx context.Context, req domain.ConvertRequest) (ConvertResult, error) {
	if strings.TrimSpace(req.SourceKey) == "" {
		return ConvertResult{}, fmt.Errorf("%w: sourceKey is required", domain.ErrInvalidInput)
	}

	now := time.Now().UTC()
	job := domain.ConvertJob{
		ID:           uuid.NewString(),
		SourceKey:    req.SourceKey,
		Format:       req.Format,
		OutputPrefix: req.OutputPrefix,
		MediaID:      req.MediaID,
		ChapterID:    req.ChapterID,
		Status:       domain.ConvertPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if job.OutputPrefix == "" {
		job.OutputPrefix = "pages/" + job.ID + "/"
	}
	if !strings.HasSuffix(job.OutputPrefix, "/") {
		job.OutputPrefix += "/"
	}

	if err := s.jobs.Create(ctx, job); err != nil {
		return ConvertResult{}, err
	}

	pages, err := s.run(ctx, &job)
	if err != nil {
		_ = s.jobs.UpdateStatus(ctx, job.ID, domain.ConvertFailed, len(pages), err.Error())
		job.Status = domain.ConvertFailed
		job.Error = err.Error()
		return ConvertResult{Job: job}, err
	}

	job.Status = domain.ConvertDone
	job.PageCount = len(pages)
	_ = s.jobs.UpdateStatus(ctx, job.ID, domain.ConvertDone, len(pages), "")
	return ConvertResult{Job: job, Pages: pages}, nil
}

// Job returns a previously-created job.
func (s *ConvertService) Job(ctx context.Context, id string) (domain.ConvertJob, error) {
	return s.jobs.Get(ctx, id)
}

func (s *ConvertService) run(ctx context.Context, job *domain.ConvertJob) ([]domain.Page, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	_ = s.jobs.UpdateStatus(ctx, job.ID, domain.ConvertRunning, 0, "")

	archive, _, err := s.store.Get(ctx, job.SourceKey)
	if err != nil {
		return nil, fmt.Errorf("fetch source: %w", err)
	}

	format, err := convert.DetectFormat(job.Format, job.SourceKey, archive)
	if err != nil {
		return nil, err
	}
	job.Format = format

	results, err := s.converter.Convert(ctx, format, archive)
	if err != nil {
		return nil, err
	}

	pages := make([]domain.Page, 0, len(results))
	stored := make([]domain.StoredPage, 0, len(results))
	for _, r := range results {
		key := fmt.Sprintf("%spage-%04d.avif", job.OutputPrefix, r.Index)
		if err := s.store.Put(ctx, key, r.Data, "image/avif"); err != nil {
			return nil, fmt.Errorf("upload page %d: %w", r.Index, err)
		}
		stored = append(stored, domain.StoredPage{Index: r.Index, R2Key: key, Width: r.Width, Height: r.Height})
		pages = append(pages, domain.Page{
			Index:    r.Index,
			ImageURL: s.publicOrProxy(key),
			Width:    r.Width,
			Height:   r.Height,
		})
	}

	// Associate produced pages with a chapter when requested.
	if job.ChapterID != "" {
		if err := s.jobs.ReplacePages(ctx, job.ChapterID, stored); err != nil {
			return nil, fmt.Errorf("persist pages: %w", err)
		}
	}
	return pages, nil
}

func (s *ConvertService) publicOrProxy(key string) string {
	if u := s.store.PublicURL(key); u != "" {
		return u
	}
	// Relative proxy path; the MangaService uses absolute URLs for reads. Here
	// we keep it relative since the convert response is informational.
	return "/v1/image?key=" + key
}
