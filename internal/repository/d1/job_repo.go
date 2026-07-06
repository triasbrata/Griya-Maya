package d1

import (
	"context"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// JobRepo persists a chapter's page rows. It is the shared page write path used
// by the presign/register ingest flow and the video/novel registration services.
type JobRepo struct {
	db Querier
}

// NewJobRepo wires a JobRepo over a D1 client.
func NewJobRepo(db *Client) *JobRepo {
	return &JobRepo{db: db}
}

// ReplacePages rewrites the page rows for a chapter after a successful ingest.
func (r *JobRepo) ReplacePages(ctx context.Context, chapterID string, pages []domain.StoredPage) error {
	if err := r.db.Exec(ctx, `DELETE FROM page WHERE chapter_id=?1`, chapterID); err != nil {
		return err
	}
	for _, p := range pages {
		kind := p.Kind
		if kind == "" {
			kind = domain.PageKindImage
		}
		if err := r.db.Exec(ctx,
			`INSERT INTO page (chapter_id, idx, r2_key, width, height, kind)
			 VALUES (?1,?2,?3,?4,?5,?6)`,
			chapterID, p.Index, p.R2Key, p.Width, p.Height, kind,
		); err != nil {
			return err
		}
	}
	return nil
}
