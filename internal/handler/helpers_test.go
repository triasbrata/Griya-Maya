package handler_test

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
)

// newCtx builds a standalone Hertz RequestContext for handler unit tests:
// method + URI (query string honored), optional path params, body and
// content-type. Handlers write into ctx.Response, which the assertions read.
func newCtx(method, uri string, params map[string]string, body []byte, contentType string) *app.RequestContext {
	c := app.NewContext(0)
	c.Request.Header.SetMethod(method)
	c.Request.SetRequestURI(uri)
	if body != nil {
		c.Request.SetBody(body)
	}
	if contentType != "" {
		c.Request.Header.SetContentTypeBytes([]byte(contentType))
	}
	for k, v := range params {
		c.Params = append(c.Params, param.Param{Key: k, Value: v})
	}
	return c
}

// decodeJSON unmarshals the response body into v.
func decodeJSON(t *testing.T, c *app.RequestContext, v any) {
	t.Helper()
	if err := json.Unmarshal(c.Response.Body(), v); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, c.Response.Body())
	}
}
