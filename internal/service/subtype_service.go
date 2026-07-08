package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// SubTypeService manages the per-type sub-type vocabulary (the `sub_type` table)
// through admin CRUD. It is backed by the same catalog repository, which owns
// the table. The public GET /v1/subtypes discovery endpoint reads the same
// vocabulary via MediaService.SubTypeCatalog.
type SubTypeService struct {
	repo MediaRepository
}

// NewSubTypeService wires a SubTypeService.
func NewSubTypeService(repo MediaRepository) *SubTypeService {
	return &SubTypeService{repo: repo}
}

// List returns every managed sub-type as a flat slice (each carrying its owning
// type), sorted by type then slug.
func (s *SubTypeService) List(ctx context.Context) ([]domain.SubType, error) {
	vocab, err := s.repo.SubTypeVocab(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SubType, 0, len(vocab))
	for t, sts := range vocab {
		for _, st := range sts {
			st.Type = t
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Slug < out[j].Slug
	})
	return out, nil
}

// Create validates and persists a new sub-type. Slug is the immutable key and
// must be unique across all types; Type must be one of manga|novel|video.
func (s *SubTypeService) Create(ctx context.Context, req domain.SubTypeWriteRequest) (domain.SubType, error) {
	st, err := subTypeFromRequest(req)
	if err != nil {
		return domain.SubType{}, err
	}
	existing, owner, err := s.find(ctx, st.Slug)
	if err != nil {
		return domain.SubType{}, err
	}
	if existing {
		return domain.SubType{}, fmt.Errorf("%w: a sub-type with slug %q already exists (type %q)", domain.ErrInvalidInput, st.Slug, owner)
	}
	if err := s.repo.CreateSubType(ctx, st); err != nil {
		return domain.SubType{}, fmt.Errorf("create sub-type: %w", err)
	}
	return st, nil
}

// Update rewrites an existing sub-type's type + name (slug is immutable).
func (s *SubTypeService) Update(ctx context.Context, slug string, req domain.SubTypeWriteRequest) (domain.SubType, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return domain.SubType{}, fmt.Errorf("%w: slug is required", domain.ErrInvalidInput)
	}
	// Keep the path slug authoritative; the body slug is ignored on update.
	req.Slug = slug
	st, err := subTypeFromRequest(req)
	if err != nil {
		return domain.SubType{}, err
	}
	exists, _, err := s.find(ctx, slug)
	if err != nil {
		return domain.SubType{}, err
	}
	if !exists {
		return domain.SubType{}, domain.ErrNotFound
	}
	if err := s.repo.UpdateSubType(ctx, slug, st); err != nil {
		return domain.SubType{}, fmt.Errorf("update sub-type: %w", err)
	}
	return st, nil
}

// Delete removes a sub-type by slug.
func (s *SubTypeService) Delete(ctx context.Context, slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return fmt.Errorf("%w: slug is required", domain.ErrInvalidInput)
	}
	return s.repo.DeleteSubType(ctx, slug)
}

// find reports whether a sub-type with the given slug exists, and its owning
// type. Slug is the primary key, so at most one row matches.
func (s *SubTypeService) find(ctx context.Context, slug string) (bool, domain.MediaType, error) {
	vocab, err := s.repo.SubTypeVocab(ctx)
	if err != nil {
		return false, "", err
	}
	for t, sts := range vocab {
		for _, st := range sts {
			if st.Slug == slug {
				return true, t, nil
			}
		}
	}
	return false, "", nil
}

// subTypeFromRequest validates a write request and builds a domain.SubType.
func subTypeFromRequest(req domain.SubTypeWriteRequest) (domain.SubType, error) {
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		return domain.SubType{}, fmt.Errorf("%w: slug is required", domain.ErrInvalidInput)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domain.SubType{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	switch req.Type {
	case domain.MediaManga, domain.MediaNovel, domain.MediaVideo:
	default:
		return domain.SubType{}, fmt.Errorf("%w: type must be manga, novel, or video", domain.ErrInvalidInput)
	}
	return domain.SubType{Slug: slug, Type: req.Type, Name: name}, nil
}
