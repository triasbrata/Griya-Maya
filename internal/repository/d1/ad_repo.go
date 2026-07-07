package d1

import (
	"context"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// AdRepo persists house-ad creatives (the `ads` table).
type AdRepo struct {
	db Querier
}

// NewAdRepo wires an AdRepo over a D1 client.
func NewAdRepo(db *Client) *AdRepo {
	return &AdRepo{db: db}
}

const adColumns = `id, r2_key, click_url, weight, placement, width, height, active, created_at`

// List returns ads ordered by weight (desc), newest first. When activeOnly is
// set only active ads are returned (the reader-facing listing); when placement
// is non-empty it filters to that placement.
func (r *AdRepo) List(ctx context.Context, activeOnly bool, placement string) ([]domain.StoredAd, error) {
	sql := `SELECT ` + adColumns + ` FROM ads`
	args := make([]any, 0, 2)
	where := ""
	if activeOnly {
		where = ` WHERE active = 1`
	}
	if placement != "" {
		if where == "" {
			where = ` WHERE placement = ?1`
		} else {
			where += ` AND placement = ?1`
		}
		args = append(args, placement)
	}
	sql += where + ` ORDER BY weight DESC, created_at DESC`
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.StoredAd, 0, len(rows))
	for _, row := range rows {
		out = append(out, adFromRow(row))
	}
	return out, nil
}

// Get returns an ad by id or domain.ErrNotFound.
func (r *AdRepo) Get(ctx context.Context, id string) (domain.StoredAd, error) {
	rows, err := r.db.Query(ctx, `SELECT `+adColumns+` FROM ads WHERE id = ?1 LIMIT 1`, id)
	if err != nil {
		return domain.StoredAd{}, err
	}
	if len(rows) == 0 {
		return domain.StoredAd{}, domain.ErrNotFound
	}
	return adFromRow(rows[0]), nil
}

// Create inserts a new ad.
func (r *AdRepo) Create(ctx context.Context, a domain.StoredAd) error {
	now := time.Now().Unix()
	return r.db.Exec(ctx,
		`INSERT INTO ads (id, r2_key, click_url, weight, placement, width, height, active, created_at)
		 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)`,
		a.ID, a.R2Key, a.ClickURL, a.Weight, a.Placement, a.Width, a.Height, boolToInt(a.Active), now)
}

// Update rewrites an ad's mutable fields (id and created_at are immutable).
func (r *AdRepo) Update(ctx context.Context, a domain.StoredAd) error {
	return r.db.Exec(ctx,
		`UPDATE ads SET r2_key=?2, click_url=?3, weight=?4, placement=?5, width=?6, height=?7, active=?8 WHERE id=?1`,
		a.ID, a.R2Key, a.ClickURL, a.Weight, a.Placement, a.Width, a.Height, boolToInt(a.Active))
}

// Delete removes an ad by id.
func (r *AdRepo) Delete(ctx context.Context, id string) error {
	return r.db.Exec(ctx, `DELETE FROM ads WHERE id = ?1`, id)
}

func adFromRow(row map[string]any) domain.StoredAd {
	return domain.StoredAd{
		ID:        strVal(row["id"]),
		R2Key:     strVal(row["r2_key"]),
		ClickURL:  strVal(row["click_url"]),
		Weight:    intVal(row["weight"]),
		Placement: strVal(row["placement"]),
		Width:     intVal(row["width"]),
		Height:    intVal(row["height"]),
		Active:    intVal(row["active"]) != 0,
		CreatedAt: timeVal(row["created_at"]),
	}
}
