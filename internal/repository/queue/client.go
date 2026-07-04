// Package queue is a Cloudflare Queues client over the HTTP REST API.
//
// Native Queue bindings are Worker-only; a Container produces and consumes via
// https://api.cloudflare.com/.../accounts/{acct}/queues/{queue_id}/messages
// (push), .../messages/pull (http_pull consumer) and .../messages/ack. The queue
// must be configured with an `http_pull` consumer. Auth is the same
// CF_ACCOUNT_ID + CF_API_TOKEN the container already uses for D1/KV (the token
// needs Queues read+write).
package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// Client is a single-queue Cloudflare Queues REST client.
type Client struct {
	http       *http.Client
	base       string // .../queues/{queue_id}
	apiToken   string
	configured bool
}

// New builds a queue client. It is inert (Configured() == false, calls error)
// when creds/queue id are missing, so the process still boots for health/docs.
func New(cfg config.QueueConfig) *Client {
	c := &Client{http: &http.Client{Timeout: 20 * time.Second}}
	if cfg.AccountID != "" && cfg.QueueID != "" && cfg.APIToken != "" {
		c.base = fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/queues/%s",
			cfg.AccountID, cfg.QueueID,
		)
		c.apiToken = cfg.APIToken
		c.configured = true
	}
	return c
}

// Configured reports whether the client can reach a queue.
func (c *Client) Configured() bool { return c.configured }

// Send pushes a single JSON message (body is JSON-encoded by the API).
func (c *Client) Send(ctx context.Context, body any) error {
	return c.post(ctx, "/messages", map[string]any{"body": body, "content_type": "json"}, nil)
}

// Message is a leased message returned by Pull. Body is the raw JSON payload.
type Message struct {
	ID       string          `json:"id"`
	LeaseID  string          `json:"lease_id"`
	Body     json.RawMessage `json:"body"`
	Attempts int             `json:"attempts"`
}

// Retry requeues a leased message after delaySeconds instead of acking it.
type Retry struct {
	LeaseID      string
	DelaySeconds int
}

// Pull leases up to batchSize messages for visibilityMS milliseconds. An empty
// slice (no error) means the queue is currently empty.
func (c *Client) Pull(ctx context.Context, batchSize, visibilityMS int) ([]Message, error) {
	var out struct {
		Result struct {
			Messages []Message `json:"messages"`
		} `json:"result"`
	}
	body := map[string]any{"batch_size": batchSize, "visibility_timeout_ms": visibilityMS}
	if err := c.post(ctx, "/messages/pull", body, &out); err != nil {
		return nil, err
	}
	return out.Result.Messages, nil
}

// Ack acknowledges (deletes) the given leases and requeues the retries.
func (c *Client) Ack(ctx context.Context, ackLeases []string, retries []Retry) error {
	acks := make([]map[string]string, 0, len(ackLeases))
	for _, l := range ackLeases {
		acks = append(acks, map[string]string{"lease_id": l})
	}
	rs := make([]map[string]any, 0, len(retries))
	for _, r := range retries {
		rs = append(rs, map[string]any{"lease_id": r.LeaseID, "delay_seconds": r.DelaySeconds})
	}
	return c.post(ctx, "/messages/ack", map[string]any{"acks": acks, "retries": rs}, nil)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	if !c.configured {
		return fmt.Errorf("queue: not configured (set CF_ACCOUNT_ID/COVER_QUEUE_ID/CF_API_TOKEN)")
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("queue post %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("queue post %s: status %d: %s", path, resp.StatusCode, b)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
