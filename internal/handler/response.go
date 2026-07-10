// Package handler is the HTTP layer: it decodes Hertz requests, invokes
// services, and encodes responses. It holds no business logic.
package handler

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// ErrorResponse documents the flat failure envelope for the swagger `@Failure`
// annotations. The wire body is always a domain.APIResponse; see writeErr /
// writeError below.
type ErrorResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	ErrorCode string `json:"error_code"`
}

// writeOK writes a typed success envelope: {"success":true,"data":<data>}.
func writeOK[T any](c *app.RequestContext, status int, data T) {
	c.JSON(status, domain.APIResponse[T]{Success: true, Data: data})
}

// writeErr writes a flat failure envelope:
// {"success":false,"error_code":<code>,"message":<message>}.
func writeErr(c *app.RequestContext, status int, code, message string) {
	c.JSON(status, domain.APIResponse[any]{Success: false, ErrorCode: code, Message: message})
}

// writeError maps a domain error to an HTTP status + failure envelope.
func writeError(c *app.RequestContext, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeErr(c, consts.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		writeErr(c, consts.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, domain.ErrUnsupportedFormat):
		writeErr(c, consts.StatusUnsupportedMediaType, "unsupported_format", err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		writeErr(c, consts.StatusUnauthorized, "unauthorized", err.Error())
	default:
		writeErr(c, consts.StatusInternalServerError, "internal", err.Error())
	}
}

// queryInt reads an integer query param with a default.
func queryInt(c *app.RequestContext, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// queryAll returns every value of a repeatable query param (e.g. ?subType=a&subType=b),
// additionally splitting comma-joined values (?subType=a,b) into individual entries.
func queryAll(c *app.RequestContext, key string) []string {
	var out []string
	c.QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) != key {
			return
		}
		for _, part := range strings.Split(string(v), ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	})
	return out
}

// parseCatalogFilter builds a domain.CatalogFilter from the shared browse/search
// query params, mirroring the app's SourceFilterValue vocabulary:
//
//	sort=popular|latest|updated|rating|title   order=asc|desc
//	type=manga|video|novel(…)   subType=<slug>(…)
//
// `type` and `subType` filter the media columns directly (both repeatable /
// comma-joinable). Category filtering was removed. `limit` overrides the page
// size (clamped by CatalogFilter.PerPage); `updated_since` restricts the feed to
// entries changed after a Unix-seconds or RFC3339 instant (for delta sync).
func parseCatalogFilter(c *app.RequestContext) domain.CatalogFilter {
	return domain.CatalogFilter{
		Sort:         c.Query("sort"),
		Ascending:    strings.EqualFold(c.Query("order"), "asc"),
		Types:        queryAll(c, "type"),
		SubTypes:     queryAll(c, "subType"),
		Limit:        queryInt(c, "limit", 0),
		UpdatedSince: parseUpdatedSince(c.Query("updated_since")),
	}
}

// parseUpdatedSince tolerantly parses the ?updated_since= filter value as either
// a Unix epoch-seconds integer or an RFC3339 timestamp. An empty or unparseable
// value yields the zero time, which disables the filter.
func parseUpdatedSince(v string) time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}
	}
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(secs, 0).UTC()
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
