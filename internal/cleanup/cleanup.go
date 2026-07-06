// Package cleanup deletes orphaned R2 objects asynchronously via a Cloudflare
// Queue. Deleting a chapter, a single page, or a media entry removes only the D1
// rows; the Producer enqueues the freed R2 keys and the Worker consumes them
// with a background pull loop that batch-deletes the objects. It mirrors the
// covermirror package (same queue client, lifecycle, and backoff shape).
package cleanup

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/repository/queue"
)

// Producer enqueues page-cleanup jobs. It satisfies service.CleanupQueue and is
// a no-op when the queue is unconfigured (cleanup disabled — objects are left in
// R2 rather than failing the delete).
type Producer struct{ q *queue.Client }

// NewProducer wires a Producer over the queue client.
func NewProducer(q *queue.Client) *Producer { return &Producer{q: q} }

// Enqueue pushes a cleanup job for the given R2 keys (no-op when there are no
// keys or the queue is off).
func (p *Producer) Enqueue(ctx context.Context, keys []string) error {
	if len(keys) == 0 || !p.q.Configured() {
		return nil
	}
	return p.q.Send(ctx, domain.PageCleanupJob{Keys: keys})
}

// ObjectDeleter batch-deletes R2 objects (implemented by r2.Store).
type ObjectDeleter interface {
	DeleteObjects(ctx context.Context, keys []string) error
}

// Worker consumes page-cleanup jobs from the queue's http_pull consumer.
type Worker struct {
	q     *queue.Client
	store ObjectDeleter
	log   *slog.Logger

	cancel context.CancelFunc
	done   chan struct{}
}

// NewWorker builds the consumer over the queue client and the R2 deleter.
func NewWorker(q *queue.Client, store ObjectDeleter) *Worker {
	return &Worker{q: q, store: store, log: slog.Default()}
}

// Start launches the pull loop (no-op if the queue is unconfigured).
func (w *Worker) Start() {
	if !w.q.Configured() {
		w.log.Info("cleanup: queue not configured; page cleanup disabled")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.loop(ctx)
	w.log.Info("cleanup: consumer started")
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
			w.log.Warn("cleanup: pull failed", "err", err)
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
			w.log.Warn("cleanup: ack failed", "err", err)
		}
	}
}

// process returns true to ack (done or unrecoverable) and false to retry.
func (w *Worker) process(ctx context.Context, m queue.Message) bool {
	var job domain.PageCleanupJob
	if err := json.Unmarshal(m.Body, &job); err != nil || len(job.Keys) == 0 {
		w.log.Warn("cleanup: dropping bad message", "err", err)
		return true // ack: poison / empty message, don't retry forever
	}
	if err := w.store.DeleteObjects(ctx, job.Keys); err != nil {
		w.log.Warn("cleanup: delete failed", "keys", len(job.Keys), "err", err)
		return false
	}
	w.log.Info("cleanup: deleted objects", "keys", len(job.Keys))
	return true
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
