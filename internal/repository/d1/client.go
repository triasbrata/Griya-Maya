// Package d1 implements a minimal client for Cloudflare D1 over its REST API.
//
// Native D1 bindings are only available to Workers; a Container reaches D1
// through https://api.cloudflare.com/.../d1/database/{id}/query.
package d1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// Client executes SQL against a single D1 database.
type Client struct {
	http       *http.Client
	endpoint   string
	apiToken   string
	configured bool
}

// New builds a D1 client. It is inert (returns errors on use) when creds are
// missing, so the process can still boot for health/docs during local dev.
func New(cfg config.D1Config) *Client {
	c := &Client{
		http: &http.Client{Timeout: 30 * time.Second},
	}
	if cfg.AccountID != "" && cfg.DatabaseID != "" && cfg.APIToken != "" {
		c.endpoint = fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
			cfg.AccountID, cfg.DatabaseID,
		)
		c.apiToken = cfg.APIToken
		c.configured = true
	}
	return c
}

type queryRequest struct {
	SQL    string `json:"sql"`
	Params []any  `json:"params,omitempty"`
}

type queryResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result []struct {
		Results []map[string]any `json:"results"`
		Success bool             `json:"success"`
		Meta    struct {
			RowsRead    int `json:"rows_read"`
			RowsWritten int `json:"rows_written"`
		} `json:"meta"`
	} `json:"result"`
}

// Query runs SQL and returns rows as maps. params are bound positionally (?1..).
func (c *Client) Query(ctx context.Context, sql string, params ...any) ([]map[string]any, error) {
	if !c.configured {
		return nil, fmt.Errorf("d1: not configured (set CF_ACCOUNT_ID/D1_DATABASE_ID/CF_API_TOKEN)")
	}

	body, err := json.Marshal(queryRequest{SQL: sql, Params: params})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("d1 request: %w", err)
	}
	defer resp.Body.Close()

	var out queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("d1 decode: %w", err)
	}
	if !out.Success || len(out.Errors) > 0 {
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("d1 error %d: %s", out.Errors[0].Code, out.Errors[0].Message)
		}
		return nil, fmt.Errorf("d1: query failed (http %d)", resp.StatusCode)
	}
	if len(out.Result) == 0 {
		return nil, nil
	}
	return out.Result[0].Results, nil
}

// Exec runs a write and ignores the (empty) result set.
func (c *Client) Exec(ctx context.Context, sql string, params ...any) error {
	_, err := c.Query(ctx, sql, params...)
	return err
}
