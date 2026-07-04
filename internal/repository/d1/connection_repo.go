package d1

import (
	"context"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// ConnectionRepo is the D1-backed store for external-source OAuth connections.
// Secret columns (client_secret/access_token/refresh_token) hold ciphertext; the
// service encrypts before writing and decrypts on use, so this layer treats them
// as opaque strings.
type ConnectionRepo struct {
	db Querier
}

// NewConnectionRepo wires a ConnectionRepo over a D1 client.
func NewConnectionRepo(db *Client) *ConnectionRepo {
	return &ConnectionRepo{db: db}
}

// connectionColumns is the shared SELECT list (stable order for row decoding).
const connectionColumns = `id, provider, label, client_id, client_secret,
        access_token, refresh_token, token_type, expires_at, status,
        created_at, updated_at`

// Create inserts a connection row (c.ID must be set).
func (r *ConnectionRepo) Create(ctx context.Context, c domain.Connection) error {
	return r.db.Exec(ctx,
		`INSERT INTO connection
		   (id, provider, label, client_id, client_secret, access_token,
		    refresh_token, token_type, expires_at, status, created_at, updated_at)
		 VALUES (?1,?2,?3,?4,?5,?6,?7,?8,?9,?10,?11,?12)`,
		c.ID, string(c.Provider), c.Label, c.ClientID, c.ClientSecret,
		c.AccessToken, c.RefreshToken, c.TokenType, c.ExpiresAt,
		string(c.Status), c.CreatedAt, c.UpdatedAt,
	)
}

// List returns all connections, newest first.
func (r *ConnectionRepo) List(ctx context.Context) ([]domain.Connection, error) {
	rows, err := r.db.Query(ctx,
		"SELECT "+connectionColumns+" FROM connection ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	out := make([]domain.Connection, 0, len(rows))
	for _, row := range rows {
		out = append(out, connectionFromRow(row))
	}
	return out, nil
}

// Get returns a single connection or domain.ErrNotFound.
func (r *ConnectionRepo) Get(ctx context.Context, id string) (domain.Connection, error) {
	rows, err := r.db.Query(ctx,
		"SELECT "+connectionColumns+" FROM connection WHERE id=?1 LIMIT 1", id)
	if err != nil {
		return domain.Connection{}, err
	}
	if len(rows) == 0 {
		return domain.Connection{}, domain.ErrNotFound
	}
	return connectionFromRow(rows[0]), nil
}

// Update rewrites the mutable credential fields (provider is fixed).
func (r *ConnectionRepo) Update(ctx context.Context, c domain.Connection) error {
	return r.db.Exec(ctx,
		`UPDATE connection SET label=?2, client_id=?3, client_secret=?4, updated_at=?5
		 WHERE id=?1`,
		c.ID, c.Label, c.ClientID, c.ClientSecret, c.UpdatedAt,
	)
}

// Delete removes a connection (idempotent).
func (r *ConnectionRepo) Delete(ctx context.Context, id string) error {
	return r.db.Exec(ctx, "DELETE FROM connection WHERE id=?1", id)
}

// SaveTokens persists the token fields after an OAuth exchange/refresh and moves
// the connection to the given status.
func (r *ConnectionRepo) SaveTokens(ctx context.Context, id, access, refresh, tokenType string, expiresAt int64, status domain.ConnectionStatus, updatedAt int64) error {
	return r.db.Exec(ctx,
		`UPDATE connection SET access_token=?2, refresh_token=?3, token_type=?4,
		    expires_at=?5, status=?6, updated_at=?7 WHERE id=?1`,
		id, access, refresh, tokenType, expiresAt, string(status), updatedAt,
	)
}

func connectionFromRow(row map[string]any) domain.Connection {
	return domain.Connection{
		ID:           strVal(row["id"]),
		Provider:     domain.Provider(strVal(row["provider"])),
		Label:        strVal(row["label"]),
		ClientID:     strVal(row["client_id"]),
		ClientSecret: strVal(row["client_secret"]),
		AccessToken:  strVal(row["access_token"]),
		RefreshToken: strVal(row["refresh_token"]),
		TokenType:    strVal(row["token_type"]),
		ExpiresAt:    int64Val(row["expires_at"]),
		Status:       domain.ConnectionStatus(strVal(row["status"])),
		CreatedAt:    int64Val(row["created_at"]),
		UpdatedAt:    int64Val(row["updated_at"]),
	}
}

// int64Val decodes a D1 numeric column (arrives as float64) to int64.
func int64Val(v any) int64 {
	return int64(floatVal(v))
}
