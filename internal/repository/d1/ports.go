package d1

import "context"

// Querier is the minimal SQL surface the repositories depend on. *Client
// satisfies it in production; tests bind a generated mock so repository logic
// (SQL shape, row mapping, error propagation) can be exercised without a real
// D1 database.
type Querier interface {
	Query(ctx context.Context, sql string, params ...any) ([]map[string]any, error)
	Exec(ctx context.Context, sql string, params ...any) error
}
