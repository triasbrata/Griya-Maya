package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// SourceService manages content sources: the reader lists enabled sources and
// admins CRUD the full set.
type SourceService struct {
	repo SourceRepository
}

// NewSourceService builds a SourceService over a source repository.
func NewSourceService(repo SourceRepository) *SourceService {
	return &SourceService{repo: repo}
}

// List returns sources. enabledOnly restricts to reader-visible ones.
func (s *SourceService) List(ctx context.Context, enabledOnly bool) ([]domain.Source, error) {
	return s.repo.List(ctx, enabledOnly)
}

// Get returns a single source by id.
func (s *SourceService) Get(ctx context.Context, id string) (domain.Source, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Source{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// Create validates and persists a new source. ID is the caller-supplied slug and
// must be unique.
func (s *SourceService) Create(ctx context.Context, req domain.SourceWriteRequest) (domain.Source, error) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return domain.Source{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(req.Name) == "" {
		return domain.Source{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	exists, err := s.repo.Exists(ctx, id)
	if err != nil {
		return domain.Source{}, err
	}
	if exists {
		return domain.Source{}, fmt.Errorf("%w: a source with id %q already exists", domain.ErrInvalidInput, id)
	}
	if err := s.repo.Create(ctx, sourceFromRequest(id, req)); err != nil {
		return domain.Source{}, fmt.Errorf("create source: %w", err)
	}
	return s.repo.Get(ctx, id)
}

// Update rewrites an existing source's mutable fields (id is immutable).
func (s *SourceService) Update(ctx context.Context, id string, req domain.SourceWriteRequest) (domain.Source, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Source{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(req.Name) == "" {
		return domain.Source{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if _, err := s.repo.Get(ctx, id); err != nil {
		return domain.Source{}, err // ErrNotFound propagates
	}
	if err := s.repo.Update(ctx, sourceFromRequest(id, req)); err != nil {
		return domain.Source{}, fmt.Errorf("update source: %w", err)
	}
	return s.repo.Get(ctx, id)
}

// Delete removes a source, refusing to orphan its catalog: a source that still
// has media must have it reassigned or deleted first.
func (s *SourceService) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	n, err := s.repo.MediaCount(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w: source still has %d media; reassign or delete them first", domain.ErrInvalidInput, n)
	}
	return s.repo.Delete(ctx, id)
}

// sourceFromRequest builds a domain.Source, applying defaults (lang "en",
// enabled true).
func sourceFromRequest(id string, req domain.SourceWriteRequest) domain.Source {
	lang := strings.TrimSpace(req.Lang)
	if lang == "" {
		lang = "en"
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return domain.Source{
		ID:      id,
		Name:    strings.TrimSpace(req.Name),
		Lang:    lang,
		IconURL: strings.TrimSpace(req.IconURL),
		Enabled: enabled,
	}
}
