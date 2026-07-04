package d1

import (
	"context"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// JobRepo persists conversion jobs and their produced pages.
type JobRepo struct {
	db Querier
}

// NewJobRepo wires a JobRepo over a D1 client.
func NewJobRepo(db *Client) *JobRepo {
	return &JobRepo{db: db}
}

// Create inserts a pending job.
func (r *JobRepo) Create(ctx context.Context, job domain.ConvertJob) error {
	return r.db.Exec(ctx,
		`INSERT INTO convert_job
		   (id, source_key, format, output_prefix, media_id, chapter_id,
		    status, page_count, error, created_at, updated_at)
		 VALUES (?1,?2,?3,?4,?5,?6,?7,?8,?9,?10,?11)`,
		job.ID, job.SourceKey, string(job.Format), job.OutputPrefix,
		job.MediaID, job.ChapterID, string(job.Status), job.PageCount,
		job.Error, job.CreatedAt.Unix(), job.UpdatedAt.Unix(),
	)
}

// UpdateStatus updates the running state, page count and error of a job.
func (r *JobRepo) UpdateStatus(ctx context.Context, id string, status domain.ConvertStatus, pageCount int, errMsg string) error {
	return r.db.Exec(ctx,
		`UPDATE convert_job SET status=?2, page_count=?3, error=?4, updated_at=?5 WHERE id=?1`,
		id, string(status), pageCount, errMsg, time.Now().Unix(),
	)
}

// Get returns a job or domain.ErrNotFound.
func (r *JobRepo) Get(ctx context.Context, id string) (domain.ConvertJob, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, source_key, format, output_prefix, media_id, chapter_id,
		        status, page_count, error, created_at, updated_at
		 FROM convert_job WHERE id=?1 LIMIT 1`, id)
	if err != nil {
		return domain.ConvertJob{}, err
	}
	if len(rows) == 0 {
		return domain.ConvertJob{}, domain.ErrNotFound
	}
	row := rows[0]
	return domain.ConvertJob{
		ID:           strVal(row["id"]),
		SourceKey:    strVal(row["source_key"]),
		Format:       domain.ArchiveFormat(strVal(row["format"])),
		OutputPrefix: strVal(row["output_prefix"]),
		MediaID:      strVal(row["media_id"]),
		ChapterID:    strVal(row["chapter_id"]),
		Status:       domain.ConvertStatus(strVal(row["status"])),
		PageCount:    intVal(row["page_count"]),
		Error:        strVal(row["error"]),
		CreatedAt:    timeVal(row["created_at"]),
		UpdatedAt:    timeVal(row["updated_at"]),
	}, nil
}

// ReplacePages rewrites the page rows for a chapter after a successful convert.
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
