package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// TaxonomyService manages the four normalized taxonomies (genre, category,
// author, artist). It is backed by the same catalog repository, which owns the
// taxonomy tables.
type TaxonomyService struct {
	repo MediaRepository
}

// NewTaxonomyService wires a TaxonomyService.
func NewTaxonomyService(repo MediaRepository) *TaxonomyService {
	return &TaxonomyService{repo: repo}
}

// List returns all tags of a kind, ordered by name.
func (s *TaxonomyService) List(ctx context.Context, kind domain.TaxonomyKind) ([]domain.Taxonomy, error) {
	if !kind.Valid() {
		return nil, fmt.Errorf("%w: unknown taxonomy kind", domain.ErrInvalidInput)
	}
	return s.repo.ListTaxonomy(ctx, kind)
}

// Create adds a tag of a kind (idempotent by slug/name).
func (s *TaxonomyService) Create(ctx context.Context, kind domain.TaxonomyKind, name string) (domain.Taxonomy, error) {
	if !kind.Valid() {
		return domain.Taxonomy{}, fmt.Errorf("%w: unknown taxonomy kind", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(name) == "" {
		return domain.Taxonomy{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	return s.repo.CreateTaxonomy(ctx, kind, strings.TrimSpace(name))
}

// Update renames a tag by id.
func (s *TaxonomyService) Update(ctx context.Context, kind domain.TaxonomyKind, id, name string) (domain.Taxonomy, error) {
	if !kind.Valid() {
		return domain.Taxonomy{}, fmt.Errorf("%w: unknown taxonomy kind", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(id) == "" {
		return domain.Taxonomy{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(name) == "" {
		return domain.Taxonomy{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	return s.repo.UpdateTaxonomy(ctx, kind, id, strings.TrimSpace(name))
}

// Delete removes a tag by id (and its media links).
func (s *TaxonomyService) Delete(ctx context.Context, kind domain.TaxonomyKind, id string) error {
	if !kind.Valid() {
		return fmt.Errorf("%w: unknown taxonomy kind", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.repo.DeleteTaxonomy(ctx, kind, id)
}
