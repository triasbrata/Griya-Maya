package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

func TestMediaHandler_ChapterNeighbors(t *testing.T) {
	h, svc, _ := newMediaHandler(t)
	prev := &domain.Chapter{ID: "c1"}
	next := &domain.Chapter{ID: "c3"}
	svc.EXPECT().ChapterNeighbors(mock.Anything, "c2").
		Return(domain.ChapterNeighbors{Previous: prev, Next: next}, nil)

	c := newCtx("GET", "/v1/chapters/c2/adjacent", map[string]string{"id": "c2"}, nil, "")
	h.ChapterNeighbors(context.Background(), c)

	var got domain.ChapterNeighbors
	decodeJSON(t, c, &got)
	require.NotNil(t, got.Previous)
	assert.Equal(t, "c1", got.Previous.ID)
	require.NotNil(t, got.Next)
	assert.Equal(t, "c3", got.Next.ID)
}

func TestMediaHandler_ChapterNeighbors_NotFound(t *testing.T) {
	h, svc, _ := newMediaHandler(t)
	svc.EXPECT().ChapterNeighbors(mock.Anything, "missing").
		Return(domain.ChapterNeighbors{}, domain.ErrNotFound)

	c := newCtx("GET", "/v1/chapters/missing/adjacent", map[string]string{"id": "missing"}, nil, "")
	h.ChapterNeighbors(context.Background(), c)

	assert.Equal(t, 404, c.Response.StatusCode())
}
