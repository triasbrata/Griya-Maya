package d1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// newTestClient points a real Client at an httptest server so Query/Exec exercise
// the full request/response path without a live D1.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(config.D1Config{AccountID: "acc", DatabaseID: "db", APIToken: "tok"})
	c.endpoint = srv.URL // override the real CF endpoint
	return c
}

func TestClient_New_InertWithoutCreds(t *testing.T) {
	c := New(config.D1Config{})
	_, err := c.Query(context.Background(), "SELECT 1")
	assert.Error(t, err) // not configured
}

func TestClient_Query_ReturnsRows(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"success":true,"result":[{"success":true,"results":[{"id":"m1"}]}]}`))
	})

	rows, err := c.Query(context.Background(), "SELECT id FROM media WHERE id=?1", "m1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "m1", rows[0]["id"])
}

func TestClient_Exec_OK(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"result":[{"success":true,"results":[]}]}`))
	})
	assert.NoError(t, c.Exec(context.Background(), "DELETE FROM media WHERE id=?1", "m1"))
}

func TestClient_Query_APIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":7500,"message":"boom"}]}`))
	})
	_, err := c.Query(context.Background(), "SELECT 1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
