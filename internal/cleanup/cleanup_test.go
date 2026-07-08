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
	called   [][]string        // keys passed to DeleteObjects
	listed   []string          // prefixes passed to ListKeys
	contents map[string][]string // prefix → keys returned by ListKeys
	err      error             // DeleteObjects error
	listErr  error             // ListKeys error
}

func (f *fakeDeleter) DeleteObjects(_ context.Context, keys []string) error {
	f.called = append(f.called, keys)
	return f.err
}

func (f *fakeDeleter) ListKeys(_ context.Context, prefix string) ([]string, error) {
	f.listed = append(f.listed, prefix)
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.contents[prefix], nil
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

func TestWorker_Process_ExpandsPrefixesThenDeletes(t *testing.T) {
	del := &fakeDeleter{contents: map[string][]string{
		"hls/vid1/": {"hls/vid1/index.m3u8", "hls/vid1/v720_init.mp4", "hls/vid1/v720_0.m4s"},
	}}
	w := NewWorker(unconfiguredClient(), del)

	body, _ := json.Marshal(domain.PageCleanupJob{
		Keys:     []string{"pages/a.avif"},
		Prefixes: []string{"hls/vid1/"},
	})
	ack := w.process(context.Background(), queue.Message{Body: body})

	assert.True(t, ack)
	assert.Equal(t, []string{"hls/vid1/"}, del.listed)
	require.Len(t, del.called, 1)
	// Explicit keys + everything listed under the prefix, one batch.
	assert.Equal(t, []string{
		"pages/a.avif",
		"hls/vid1/index.m3u8", "hls/vid1/v720_init.mp4", "hls/vid1/v720_0.m4s",
	}, del.called[0])
}

func TestWorker_Process_ListFailureRetries(t *testing.T) {
	del := &fakeDeleter{listErr: errors.New("r2 list down")}
	w := NewWorker(unconfiguredClient(), del)

	body, _ := json.Marshal(domain.PageCleanupJob{Prefixes: []string{"hls/vid1/"}})
	ack := w.process(context.Background(), queue.Message{Body: body})

	assert.False(t, ack)       // transient → retry
	assert.Empty(t, del.called) // never reached delete
}

func TestWorker_Process_EmptyPrefixListingIsAcked(t *testing.T) {
	del := &fakeDeleter{contents: map[string][]string{"hls/gone/": nil}}
	w := NewWorker(unconfiguredClient(), del)

	body, _ := json.Marshal(domain.PageCleanupJob{Prefixes: []string{"hls/gone/"}})
	ack := w.process(context.Background(), queue.Message{Body: body})

	assert.True(t, ack)         // already-cleaned prefix → nothing to delete
	assert.Empty(t, del.called) // no delete call for an empty key set
}

func TestProducer_EnqueuePrefixes_Noops(t *testing.T) {
	p := NewProducer(unconfiguredClient())
	require.NoError(t, p.EnqueuePrefixes(context.Background(), nil))
	require.NoError(t, p.EnqueuePrefixes(context.Background(), []string{"hls/x/"}))
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
