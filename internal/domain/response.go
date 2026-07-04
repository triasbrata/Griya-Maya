package domain

// APIResponse is the uniform, generic envelope wrapping every REST API response
// body, so clients can rely on a single shape: inspect `success`, then read the
// typed `data` on success or the flat `message` + `error_code` on failure. It
// intentionally does NOT wrap the OAuth/OIDC protocol endpoints (token,
// discovery, JWKS, dynamic client registration), which are bound to their own
// RFC-defined formats.
//
// ErrorCode is a stable, snake_case token (e.g. "not_found"); Message is
// human-readable. Both are omitted on success; Data is omitted on failure.
type APIResponse[T any] struct {
	Success   bool   `json:"success"`
	Data      T      `json:"data,omitempty"`
	Message   string `json:"message,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}
