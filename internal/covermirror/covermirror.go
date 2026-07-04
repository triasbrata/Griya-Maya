// Package covermirror mirrors external media cover images into R2 as AVIF,
// asynchronously via a Cloudflare Queue. The Producer enqueues jobs (from the
// media service on create/update); the Worker consumes them with a background
// pull loop: fetch the image, AVIF-encode it, upload to R2, and rewrite
// media.cover_url to the stored key.
package covermirror

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/convert"
	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/repository/queue"
)

// maxCoverBytes caps how much of a remote image we download.
const maxCoverBytes = 25 << 20 // 25 MiB

// Producer enqueues cover-mirror jobs. It satisfies service.CoverMirrorQueue and
// is a no-op when the queue is unconfigured (mirror disabled).
type Producer struct{ q *queue.Client }

// NewProducer wires a Producer over the queue client.
func NewProducer(q *queue.Client) *Producer { return &Producer{q: q} }

// Enqueue pushes a mirror job (no-op when the queue is off).
func (p *Producer) Enqueue(ctx context.Context, job domain.CoverMirrorJob) error {
	if !p.q.Configured() {
		return nil
	}
	return p.q.Send(ctx, job)
}

// ObjectPutter stores the encoded cover (implemented by r2.Store).
type ObjectPutter interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
}

// CoverUpdater rewrites media.cover_url (implemented by d1.MediaRepo).
type CoverUpdater interface {
	SetMediaCover(ctx context.Context, mediaID, coverURL string) error
}

// Worker consumes cover-mirror jobs from the queue's http_pull consumer.
type Worker struct {
	q     *queue.Client
	store ObjectPutter
	repo  CoverUpdater
	http  *http.Client
	opt   convert.EncodeOptions
	log   *slog.Logger

	cancel context.CancelFunc
	done   chan struct{}
}

// NewWorker builds the consumer. opt controls AVIF quality/speed/downscale.
func NewWorker(q *queue.Client, store ObjectPutter, repo CoverUpdater, opt convert.EncodeOptions) *Worker {
	return &Worker{
		q:     q,
		store: store,
		repo:  repo,
		http:  &http.Client{Timeout: 30 * time.Second},
		opt:   opt,
		log:   slog.Default(),
	}
}

// Start launches the pull loop (no-op if the queue is unconfigured).
func (w *Worker) Start() {
	if !w.q.Configured() {
		w.log.Info("covermirror: queue not configured; cover mirror disabled")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.loop(ctx)
	w.log.Info("covermirror: consumer started")
}

// Stop signals the loop to exit and waits for it to drain.
func (w *Worker) Stop() {
	if w.cancel == nil {
		return
	}
	w.cancel()
	<-w.done
}

func (w *Worker) loop(ctx context.Context) {
	defer close(w.done)
	const batchSize = 10
	const visibilityMS = 60_000
	for ctx.Err() == nil {
		msgs, err := w.q.Pull(ctx, batchSize, visibilityMS)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.log.Warn("covermirror: pull failed", "err", err)
			w.sleep(ctx, 5*time.Second)
			continue
		}
		if len(msgs) == 0 {
			w.sleep(ctx, 2*time.Second)
			continue
		}
		var acks []string
		var retries []queue.Retry
		for _, m := range msgs {
			if w.process(ctx, m) {
				acks = append(acks, m.LeaseID)
			} else {
				retries = append(retries, queue.Retry{LeaseID: m.LeaseID, DelaySeconds: backoff(m.Attempts)})
			}
		}
		if err := w.q.Ack(ctx, acks, retries); err != nil {
			w.log.Warn("covermirror: ack failed", "err", err)
		}
	}
}

// process returns true to ack (done or unrecoverable) and false to retry.
func (w *Worker) process(ctx context.Context, m queue.Message) bool {
	var job domain.CoverMirrorJob
	if err := json.Unmarshal(m.Body, &job); err != nil || job.MediaID == "" || job.SourceURL == "" {
		w.log.Warn("covermirror: dropping bad message", "err", err)
		return true // ack: poison message, don't retry forever
	}
	raw, err := w.fetch(ctx, job.SourceURL)
	if err != nil {
		w.log.Warn("covermirror: fetch failed", "url", job.SourceURL, "err", err)
		return false
	}
	encoded, err := convert.EncodeImage(raw, w.opt)
	if err != nil {
		// A non-image / undecodable URL will never succeed — ack to stop retrying.
		w.log.Warn("covermirror: encode failed, dropping", "media", job.MediaID, "err", err)
		return true
	}
	key := "covers/" + job.MediaID + ".avif"
	if err := w.store.Put(ctx, key, encoded, "image/avif"); err != nil {
		w.log.Warn("covermirror: r2 put failed", "media", job.MediaID, "err", err)
		return false
	}
	if err := w.repo.SetMediaCover(ctx, job.MediaID, key); err != nil {
		w.log.Warn("covermirror: cover update failed", "media", job.MediaID, "err", err)
		return false
	}
	w.log.Info("covermirror: mirrored cover", "media", job.MediaID, "key", key)
	return true
}

func (w *Worker) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover fetch: status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxCoverBytes))
}

func (w *Worker) sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// backoff grows the retry delay with attempts: 30s, 60s, 120s, … capped at 1h.
func backoff(attempts int) int {
	if attempts < 0 {
		attempts = 0
	}
	d := 30 << min(attempts, 7)
	return min(d, 3600)
}
