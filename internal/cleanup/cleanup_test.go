package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/config"
	"github.com/triasbrata/mihon-manga-server/internal/domain"
	"github.com/triasbrata/mihon-manga-server/internal/repository/queue"
)

// fakeDeleter records the keys it was asked to delete and can fail on demand.
type fakeDeleter struct {
	called [][]string
	err    error
}

func (f *fakeDeleter) DeleteObjects(_ context.Context, keys []string) error {
	f.called = append(f.called, keys)
	return f.err
}

// unconfiguredClient is a queue client with no creds → Configured() == false.
func unconfiguredClient() *queue.Client { return queue.New(config.QueueConfig{}) }

func TestProducer_Enqueue_NoKeysIsNoop(t *testing.T) {
	p := NewProducer(unconfiguredClient())
	require.NoError(t, p.Enqueue(context.Background(), nil))
}

func TestProducer_Enqueue_UnconfiguredIsNoop(t *testing.T) {
	// With keys but no queue, Enqueue silently no-ops (does not error the delete).
	p := NewProducer(unconfiguredClient())
	require.NoError(t, p.Enqueue(context.Background(), []string{"pages/a.avif"}))
}

func TestWorker_Process_DeletesKeys(t *testing.T) {
	del := &fakeDeleter{}
	w := NewWorker(unconfiguredClient(), del)

	body, _ := json.Marshal(domain.PageCleanupJob{Keys: []string{"pages/a.avif", "pages/b.avif"}})
	ack := w.process(context.Background(), queue.Message{Body: body})

	assert.True(t, ack)
	require.Len(t, del.called, 1)
	assert.Equal(t, []string{"pages/a.avif", "pages/b.avif"}, del.called[0])
}

func TestWorker_Process_BadJSONIsAcked(t *testing.T) {
	del := &fakeDeleter{}
	w := NewWorker(unconfiguredClient(), del)

	ack := w.process(context.Background(), queue.Message{Body: []byte("{not json")})

	assert.True(t, ack) // poison message → ack, never retried
	assert.Empty(t, del.called)
}

func TestWorker_Process_EmptyKeysIsAcked(t *testing.T) {
	del := &fakeDeleter{}
	w := NewWorker(unconfiguredClient(), del)

	body, _ := json.Marshal(domain.PageCleanupJob{Keys: nil})
	ack := w.process(context.Background(), queue.Message{Body: body})

	assert.True(t, ack)
	assert.Empty(t, del.called)
}

func TestWorker_Process_DeleteErrorRetries(t *testing.T) {
	del := &fakeDeleter{err: errors.New("r2 down")}
	w := NewWorker(unconfiguredClient(), del)

	body, _ := json.Marshal(domain.PageCleanupJob{Keys: []string{"pages/a.avif"}})
	ack := w.process(context.Background(), queue.Message{Body: body})

	assert.False(t, ack) // transient failure → retry
}

func TestWorker_StartStop_NoopWhenUnconfigured(t *testing.T) {
	w := NewWorker(unconfiguredClient(), &fakeDeleter{})
	// Start is a no-op (queue off); Stop must be safe with no loop running.
	w.Start()
	w.Stop()
}

func TestBackoff(t *testing.T) {
	assert.Equal(t, 30, backoff(0))
	assert.Equal(t, 60, backoff(1))
	assert.Equal(t, 30, backoff(-5))  // clamped to 0
	assert.Equal(t, 3600, backoff(20)) // capped at 1h
}
