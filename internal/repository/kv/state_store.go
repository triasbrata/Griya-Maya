package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// statePrefix namespaces OAuth PKCE/state entries in the shared KV namespace.
const statePrefix = "conn:oauth:state:"

// StateStore persists the short-lived PKCE/state bundle between the authorize
// redirect and the callback, keyed by the opaque `state`. It is backed by the
// same KV namespace used for OIDC state and satisfies the service's StateStore
// port structurally.
type StateStore struct {
	kv *Client
}

// NewStateStore wraps a KV client as an OAuth state store.
func NewStateStore(kv *Client) *StateStore {
	return &StateStore{kv: kv}
}

// Put stores v under state with a TTL (KV enforces a 60s floor).
func (s *StateStore) Put(ctx context.Context, state string, v domain.AuthState, ttlSeconds int) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.kv.Put(ctx, statePrefix+state, data, time.Duration(ttlSeconds)*time.Second)
}

// Get returns the AuthState for state and deletes it (single-use). A missing or
// expired state maps to domain.ErrNotFound.
func (s *StateStore) Get(ctx context.Context, state string) (domain.AuthState, error) {
	data, err := s.kv.Get(ctx, statePrefix+state)
	if err != nil {
		if err == ErrNotFound {
			return domain.AuthState{}, domain.ErrNotFound
		}
		return domain.AuthState{}, fmt.Errorf("oauth state get: %w", err)
	}
	var v domain.AuthState
	if err := json.Unmarshal(data, &v); err != nil {
		return domain.AuthState{}, fmt.Errorf("oauth state decode: %w", err)
	}
	// Single-use: best-effort delete so a state cannot be replayed.
	_ = s.kv.Delete(ctx, statePrefix+state)
	return v, nil
}
