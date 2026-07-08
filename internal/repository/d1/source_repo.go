package d1

import (
	"context"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// SourceRepo persists content sources (the normalized `source` table).
type SourceRepo struct {
	db Querier
}

// NewSourceRepo wires a SourceRepo over a D1 client.
func NewSourceRepo(db *Client) *SourceRepo {
	return &SourceRepo{db: db}
}

const sourceColumns = `id, name, lang, icon_url, enabled, created_at, updated_at`

// List returns sources ordered by name. When enabledOnly is set, only enabled
// sources are returned (the reader-facing listing).
func (r *SourceRepo) List(ctx context.Context, enabledOnly bool) ([]domain.Source, error) {
	sql := `SELECT ` + sourceColumns + ` FROM source`
	if enabledOnly {
		sql += ` WHERE enabled = 1`
	}
	sql += ` ORDER BY name`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Source, 0, len(rows))
	for _, row := range rows {
		out = append(out, sourceFromRow(row))
	}
	types, err := r.mediaTypesBySource(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if ts := types[out[i].ID]; len(ts) > 0 {
			out[i].MediaTypes = ts
		}
	}
	return out, nil
}

// mediaTypesBySource returns, per source id, the distinct media types it carries
// (alphabetically sorted). A single grouped scan of `media` avoids an N+1 probe
// per source; the SQL does the dedup and ordering so the response is stable.
func (r *SourceRepo) mediaTypesBySource(ctx context.Context) (map[string][]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT source_id, type FROM media
		 WHERE type IS NOT NULL AND type <> ''
		 GROUP BY source_id, type
		 ORDER BY source_id, type ASC`)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(rows))
	for _, row := range rows {
		sid := strVal(row["source_id"])
		if sid == "" {
			continue
		}
		out[sid] = append(out[sid], strVal(row["type"]))
	}
	return out, nil
}

// Get returns a source by id or domain.ErrNotFound.
func (r *SourceRepo) Get(ctx context.Context, id string) (domain.Source, error) {
	rows, err := r.db.Query(ctx, `SELECT `+sourceColumns+` FROM source WHERE id = ?1 LIMIT 1`, id)
	if err != nil {
		return domain.Source{}, err
	}
	if len(rows) == 0 {
		return domain.Source{}, domain.ErrNotFound
	}
	return sourceFromRow(rows[0]), nil
}

// Exists reports whether a source id is already taken.
func (r *SourceRepo) Exists(ctx context.Context, id string) (bool, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM source WHERE id = ?1 LIMIT 1`, id)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

// MediaCount returns how many media rows reference the source (used to refuse
// deleting a non-empty source).
func (r *SourceRepo) MediaCount(ctx context.Context, id string) (int, error) {
	rows, err := r.db.Query(ctx, `SELECT count(*) AS n FROM media WHERE source_id = ?1`, id)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return intVal(rows[0]["n"]), nil
}

// Create inserts a new source.
func (r *SourceRepo) Create(ctx context.Context, s domain.Source) error {
	now := time.Now().Unix()
	return r.db.Exec(ctx,
		`INSERT INTO source (id, name, lang, icon_url, enabled, created_at, updated_at)
		 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?6)`,
		s.ID, s.Name, s.Lang, s.IconURL, boolToInt(s.Enabled), now)
}

// Update rewrites a source's mutable fields (id is immutable).
func (r *SourceRepo) Update(ctx context.Context, s domain.Source) error {
	return r.db.Exec(ctx,
		`UPDATE source SET name=?2, lang=?3, icon_url=?4, enabled=?5, updated_at=?6 WHERE id=?1`,
		s.ID, s.Name, s.Lang, s.IconURL, boolToInt(s.Enabled), time.Now().Unix())
}

// Delete removes a source by id.
func (r *SourceRepo) Delete(ctx context.Context, id string) error {
	return r.db.Exec(ctx, `DELETE FROM source WHERE id = ?1`, id)
}

func sourceFromRow(row map[string]any) domain.Source {
	return domain.Source{
		ID:        strVal(row["id"]),
		Name:      strVal(row["name"]),
		Lang:      strVal(row["lang"]),
		IconURL:   strVal(row["icon_url"]),
		Enabled:   intVal(row["enabled"]) != 0,
		CreatedAt: timeVal(row["created_at"]),
		UpdatedAt: timeVal(row["updated_at"]),
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
