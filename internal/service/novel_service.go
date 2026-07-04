package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// NovelService registers a text chapter (stored as a `.txt` in R2) as a
// chapter's single novel page. Inline text is written to R2 here; an already-
// uploaded object can be referenced by key instead.
type NovelService struct {
	pages JobRepository
	store ObjectStore
}

// NewNovelService wires a NovelService. It reuses JobRepository.ReplacePages to
// persist the (single) novel page for a chapter.
func NewNovelService(pages JobRepository, store ObjectStore) *NovelService {
	return &NovelService{pages: pages, store: store}
}

// Register stores/associates a text chapter with a chapter, rewriting its pages
// to a single novel page. Returns the resulting page (with Body) for confirmation.
func (s *NovelService) Register(ctx context.Context, req domain.NovelRegisterRequest) (domain.Page, error) {
	chapterID := strings.TrimSpace(req.ChapterID)
	if chapterID == "" {
		return domain.Page{}, fmt.Errorf("%w: chapterId is required", domain.ErrInvalidInput)
	}

	key := strings.TrimLeft(strings.TrimSpace(req.TextKey), "/")
	body := req.Text
	switch {
	case key != "":
		// Reference an already-uploaded object; fetch it back to echo Body.
		data, _, err := s.store.Get(ctx, key)
		if err != nil {
			return domain.Page{}, fmt.Errorf("fetch novel text: %w", err)
		}
		body = string(data)
	case req.Text != "":
		key = "novels/" + uuid.NewString() + "/chapter.txt"
		if err := s.store.Put(ctx, key, []byte(req.Text), "text/plain; charset=utf-8"); err != nil {
			return domain.Page{}, fmt.Errorf("store novel text: %w", err)
		}
	default:
		return domain.Page{}, fmt.Errorf("%w: one of text or textKey is required", domain.ErrInvalidInput)
	}

	stored := []domain.StoredPage{{
		Index: 0,
		R2Key: key,
		Kind:  domain.PageKindNovel,
	}}
	if err := s.pages.ReplacePages(ctx, chapterID, stored); err != nil {
		return domain.Page{}, fmt.Errorf("persist novel page: %w", err)
	}

	return domain.Page{
		Index: 0,
		Type:  domain.PageKindNovel,
		Body:  body,
	}, nil
}
