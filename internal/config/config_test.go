package config_test

import (
	"testing"

	"github.com/triasbrata/mihon-manga-server/internal/config"
)

// TestLoad_R2PublicBaseURL_Sanitized verifies that R2_PUBLIC_BASE_URL only
// survives into the config when it is a valid absolute http(s) URL; anything
// malformed (empty, whitespace, a leaked config comment, a scheme-less value)
// collapses to "" so page URLs fall back to the private presigned/proxy path.
func TestLoad_R2PublicBaseURL_Sanitized(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace", "   ", ""},
		{"valid_https", "https://cdn.example.com", "https://cdn.example.com"},
		{"valid_http", "http://localhost:9000", "http://localhost:9000"},
		{"trailing_slash_trimmed", "https://cdn.example.com/", "https://cdn.example.com"},
		{"leaked_comment", "# e.g. https://cdn.example.com (empty => proxy via /v1/image)", ""},
		{"comment_body_with_spaces", "https://cdn.example.com (empty => proxy via /v1/image)", ""},
		{"no_scheme", "cdn.example.com", ""},
		{"ftp_scheme", "ftp://cdn.example.com", ""},
		{"scheme_no_host", "https://", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A 32-byte conn key keeps Load() from failing validation.
			t.Setenv("CONNECTIONS_ENC_KEY", "0123456789abcdef0123456789abcdef")
			t.Setenv("R2_PUBLIC_BASE_URL", tc.in)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := cfg.R2.PublicBaseURL; got != tc.want {
				t.Fatalf("R2.PublicBaseURL = %q, want %q", got, tc.want)
			}
		})
	}
}
