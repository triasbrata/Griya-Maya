package domain

import "errors"

// Common typed errors surfaced to handlers for status mapping.
var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUnsupportedFormat = errors.New("unsupported archive format")
	ErrUnauthorized      = errors.New("unauthorized")
)
