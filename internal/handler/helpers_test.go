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

// decodeJSON unmarshals the `data` field of the uniform success envelope
// ({"success":true,"data":…}) into v. All success responses share that shape.
func decodeJSON(t *testing.T, c *app.RequestContext, v any) {
	t.Helper()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(c.Response.Body(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%s)", err, c.Response.Body())
	}
	if err := json.Unmarshal(env.Data, v); err != nil {
		t.Fatalf("decode data: %v (body=%s)", err, c.Response.Body())
	}
}

// apiError mirrors the flat failure envelope fields for test assertions.
type apiError struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// decodeError unmarshals the flat failure envelope
// ({"success":false,"error_code":…,"message":…}).
func decodeError(t *testing.T, c *app.RequestContext) apiError {
	t.Helper()
	var e apiError
	if err := json.Unmarshal(c.Response.Body(), &e); err != nil {
		t.Fatalf("decode error envelope: %v (body=%s)", err, c.Response.Body())
	}
	return e
}
