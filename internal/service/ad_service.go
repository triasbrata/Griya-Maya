package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// AdService manages house-ad creatives: the reader lists active ads for a
// placement (each mapped to a presigned image URL), and admins CRUD the full set
// plus mint presigned upload URLs for new creatives.
type AdService struct {
	repo       AdRepository
	store      ObjectStore
	presignTTL time.Duration
}

// NewAdService wires an AdService. presignTTL bounds how long the direct R2
// creative URLs it mints for the reader stay valid (shared with page URLs).
func NewAdService(repo AdRepository, store ObjectStore, presignTTL time.Duration) *AdService {
	return &AdService{repo: repo, store: store, presignTTL: presignTTL}
}

// adPresignPutTTL bounds how long a minted creative-upload URL stays valid.
const adPresignPutTTL = 30 * time.Minute

// ListActive returns the active ads for the reader, each with a freshly presigned
// image URL. An empty placement returns all active ads across placements.
func (s *AdService) ListActive(ctx context.Context, placement string) ([]domain.Ad, error) {
	stored, err := s.repo.List(ctx, true, strings.TrimSpace(placement))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Ad, 0, len(stored))
	for _, a := range stored {
		url, err := s.store.PresignGet(ctx, a.R2Key, s.presignTTL)
		if err != nil {
			return nil, fmt.Errorf("presign ad %q: %w", a.ID, err)
		}
		out = append(out, domain.Ad{
			ID:          a.ID,
			ImageURL:    url,
			ClickURL:    a.ClickURL,
			Weight:      a.Weight,
			Placement:   a.Placement,
			AspectRatio: aspectRatio(a.Width, a.Height),
			Width:       a.Width,
			Height:      a.Height,
		})
	}
	return out, nil
}

// List returns every ad (active and inactive) for the admin surface.
func (s *AdService) List(ctx context.Context) ([]domain.StoredAd, error) {
	return s.repo.List(ctx, false, "")
}

// Get returns a single ad by id.
func (s *AdService) Get(ctx context.Context, id string) (domain.StoredAd, error) {
	if strings.TrimSpace(id) == "" {
		return domain.StoredAd{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.repo.Get(ctx, id)
}

// Create validates and persists a new ad, returning the stored row.
func (s *AdService) Create(ctx context.Context, req domain.AdWriteRequest) (domain.StoredAd, error) {
	if strings.TrimSpace(req.R2Key) == "" {
		return domain.StoredAd{}, fmt.Errorf("%w: r2Key is required", domain.ErrInvalidInput)
	}
	ad := adFromRequest(uuid.NewString(), req)
	if err := s.repo.Create(ctx, ad); err != nil {
		return domain.StoredAd{}, fmt.Errorf("create ad: %w", err)
	}
	return s.repo.Get(ctx, ad.ID)
}

// Update rewrites an existing ad's mutable fields (id is immutable).
func (s *AdService) Update(ctx context.Context, id string, req domain.AdWriteRequest) (domain.StoredAd, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.StoredAd{}, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(req.R2Key) == "" {
		return domain.StoredAd{}, fmt.Errorf("%w: r2Key is required", domain.ErrInvalidInput)
	}
	if _, err := s.repo.Get(ctx, id); err != nil {
		return domain.StoredAd{}, err // ErrNotFound propagates
	}
	ad := adFromRequest(id, req)
	if err := s.repo.Update(ctx, ad); err != nil {
		return domain.StoredAd{}, fmt.Errorf("update ad: %w", err)
	}
	return s.repo.Get(ctx, id)
}

// Delete removes an ad by id.
func (s *AdService) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.repo.Delete(ctx, id)
}

// PresignUpload mints a single presigned R2 PUT URL under a fresh ads/ key so the
// admin browser uploads a creative straight to R2. contentType defaults to
// "image/avif" when empty. The returned Key is what the admin sends back as the
// AdWriteRequest.R2Key on create/update.
func (s *AdService) PresignUpload(ctx context.Context, contentType string) (PresignItem, error) {
	if contentType == "" {
		contentType = "image/avif"
	}
	key := "ads/" + uuid.NewString()
	url, err := s.store.PresignPut(ctx, key, adPresignPutTTL, contentType)
	if err != nil {
		return PresignItem{}, fmt.Errorf("presign ad upload: %w", err)
	}
	return PresignItem{Key: key, URL: url}, nil
}

// adFromRequest builds a domain.StoredAd, applying defaults (weight 1, active
// true) when the request omits them.
func adFromRequest(id string, req domain.AdWriteRequest) domain.StoredAd {
	weight := req.Weight
	if weight <= 0 {
		weight = 1
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	return domain.StoredAd{
		ID:        id,
		R2Key:     strings.TrimSpace(req.R2Key),
		ClickURL:  strings.TrimSpace(req.ClickURL),
		Weight:    weight,
		Placement: strings.TrimSpace(req.Placement),
		Width:     req.Width,
		Height:    req.Height,
		Active:    active,
	}
}

// aspectRatio returns width/height when both are positive, else 0 (omitted).
func aspectRatio(w, h int) float64 {
	if w <= 0 || h <= 0 {
		return 0
	}
	return float64(w) / float64(h)
}
