// Package handler is the HTTP layer: it decodes Hertz requests, invokes
// services, and encodes responses. It holds no business logic.
package handler

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// ErrorResponse is the uniform error body.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// writeError maps a domain error to an HTTP status + JSON body.
func writeError(c *app.RequestContext, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		c.JSON(consts.StatusNotFound, ErrorResponse{Error: "not_found", Message: err.Error()})
	case errors.Is(err, domain.ErrInvalidInput):
		c.JSON(consts.StatusBadRequest, ErrorResponse{Error: "invalid_input", Message: err.Error()})
	case errors.Is(err, domain.ErrUnsupportedFormat):
		c.JSON(consts.StatusUnsupportedMediaType, ErrorResponse{Error: "unsupported_format", Message: err.Error()})
	case errors.Is(err, domain.ErrUnauthorized):
		c.JSON(consts.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: err.Error()})
	default:
		c.JSON(consts.StatusInternalServerError, ErrorResponse{Error: "internal", Message: err.Error()})
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

// queryAll returns every value of a repeatable query param (e.g. ?genre=a&genre=b),
// additionally splitting comma-joined values (?genre=a,b) into individual entries.
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
//	type=manga|video|novel(…)   genre=<slug>(…)   genreExclude=<slug>(…)
//	category=<slug>(…)   categoryExclude=<slug>(…)   genreMode=and|or
//
// `type` filters the media kind column directly; `category` filters the
// normalized category taxonomy (both repeatable / comma-joinable).
func parseCatalogFilter(c *app.RequestContext) domain.CatalogFilter {
	mode := domain.GenreModeOr
	if strings.EqualFold(c.Query("genreMode"), "and") {
		mode = domain.GenreModeAnd
	}
	return domain.CatalogFilter{
		Sort:              c.Query("sort"),
		Ascending:         strings.EqualFold(c.Query("order"), "asc"),
		Types:             queryAll(c, "type"),
		IncludeGenres:     queryAll(c, "genre"),
		ExcludeGenres:     queryAll(c, "genreExclude"),
		IncludeCategories: queryAll(c, "category"),
		ExcludeCategories: queryAll(c, "categoryExclude"),
		GenreMode:         mode,
	}
}
