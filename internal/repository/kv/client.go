// Package kv implements a client for Cloudflare Workers KV over its REST API.
//
// Native KV bindings are only available to Workers; a Container reaches KV via
// https://api.cloudflare.com/.../storage/kv/namespaces/{id}/values/{key}.
// KV's native per-key expiration_ttl is used for short-lived OIDC state
// (auth requests, auth codes, access tokens).
package kv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// ErrNotFound is returned when a key is absent (or expired).
var ErrNotFound = errors.New("kv: key not found")

// Client is a namespace-scoped Cloudflare KV client.
type Client struct {
	http       *http.Client
	base       string // .../storage/kv/namespaces/{id}
	apiToken   string
	configured bool
}

// New builds a KV client. It is inert (returns errors on use) when creds are
// missing, so the process can still boot for health/docs during local dev.
func New(cfg config.KVConfig) *Client {
	c := &Client{http: &http.Client{Timeout: 15 * time.Second}}
	if cfg.AccountID != "" && cfg.NamespaceID != "" && cfg.APIToken != "" {
		c.base = fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/storage/kv/namespaces/%s",
			cfg.AccountID, cfg.NamespaceID,
		)
		c.apiToken = cfg.APIToken
		c.configured = true
	}
	return c
}

func (c *Client) keyURL(key string) string {
	return c.base + "/values/" + url.PathEscape(key)
}

// Get returns the raw value for key, or ErrNotFound.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	if !c.configured {
		return nil, fmt.Errorf("kv: not configured (set CF_ACCOUNT_ID/KV_NAMESPACE_ID/CF_API_TOKEN)")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.keyURL(key), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kv get: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return io.ReadAll(resp.Body)
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("kv get: unexpected status %d", resp.StatusCode)
	}
}

// Put stores value under key. A non-zero ttl sets expiration_ttl (min 60s per
// Cloudflare); zero means no expiration.
func (c *Client) Put(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !c.configured {
		return fmt.Errorf("kv: not configured")
	}
	u := c.keyURL(key)
	if secs := int(ttl.Seconds()); secs >= 60 {
		u += "?expiration_ttl=" + strconv.Itoa(secs)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(value))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("kv put: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kv put: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Delete removes a key (idempotent).
func (c *Client) Delete(ctx context.Context, key string) error {
	if !c.configured {
		return fmt.Errorf("kv: not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.keyURL(key), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("kv delete: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("kv delete: unexpected status %d", resp.StatusCode)
	}
	return nil
}
