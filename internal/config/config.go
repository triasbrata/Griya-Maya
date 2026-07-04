// Package config loads runtime configuration from the environment.
//
// The server is designed to run inside a Cloudflare Container, so it reaches
// D1 over the REST API and R2 over the S3-compatible API. All credentials are
// injected as environment variables (see .env.example).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	HTTP  HTTPConfig
	D1    D1Config
	R2    R2Config
	KV    KVConfig
	OIDC  OIDCConfig
	Image ImageConfig
}

// HTTPConfig controls the Hertz server.
type HTTPConfig struct {
	// Addr is the listen address. Cloudflare Containers expect the app to
	// listen on the port advertised via wrangler (default 8080).
	Addr string
	// PublicBaseURL is the externally reachable base URL (through the fronting
	// Worker). Page image URLs handed to the app are built against it.
	PublicBaseURL string
}

// D1Config addresses a Cloudflare D1 database via the REST API.
type D1Config struct {
	AccountID  string
	DatabaseID string
	APIToken   string
}

// R2Config addresses a Cloudflare R2 bucket via the S3-compatible API.
type R2Config struct {
	AccountID       string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	// PublicBaseURL is the R2 public/custom-domain base used to build page URLs
	// (e.g. https://cdn.example.com). Empty means serve through this service.
	PublicBaseURL string
}

// KVConfig addresses a Cloudflare Workers KV namespace via the REST API. It
// holds short-lived OIDC state (auth requests, codes, access tokens) with TTL.
type KVConfig struct {
	AccountID   string
	NamespaceID string
	APIToken    string
}

// OIDCConfig configures the embedded OpenID Provider (zitadel/oidc).
type OIDCConfig struct {
	// Issuer is the externally reachable base URL of this provider (usually the
	// same as HTTP.PublicBaseURL). It appears in tokens as `iss`.
	Issuer string
	// CryptoKey is a 32-byte key (hex or raw) used by the OP to encrypt tokens.
	CryptoKey string
	// RequiredScope, when set, is enforced on protected manga API routes.
	RequiredScope string
	// Token lifetimes.
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	AuthCodeTTL     time.Duration
	// Seed admin (created on first boot if the admin_user table is empty).
	AdminEmail    string
	AdminPassword string
	// AdminRedirectURIs are the redirect_uris for the seeded static `admin-web`
	// PKCE client (comma-separated in ADMIN_REDIRECT_URIS).
	AdminRedirectURIs []string
}

// ImageConfig tunes AVIF conversion output.
type ImageConfig struct {
	// Quality is the AVIF quality (0-100).
	Quality int
	// Speed is the AVIF encoder speed (0 slowest/best .. 10 fastest).
	Speed int
	// MaxEdge downscales pages whose longest edge exceeds it (0 = no cap).
	MaxEdge int
	// ConvertTimeout bounds a single archive conversion.
	ConvertTimeout time.Duration
}

// Load reads configuration from the environment, applying defaults.
func Load() (Config, error) {
	cfg := Config{
		HTTP: HTTPConfig{
			Addr:          env("HTTP_ADDR", ":8080"),
			PublicBaseURL: env("PUBLIC_BASE_URL", "http://localhost:8080"),
		},
		D1: D1Config{
			AccountID:  os.Getenv("CF_ACCOUNT_ID"),
			DatabaseID: os.Getenv("D1_DATABASE_ID"),
			APIToken:   os.Getenv("CF_API_TOKEN"),
		},
		R2: R2Config{
			AccountID:       os.Getenv("CF_ACCOUNT_ID"),
			Bucket:          env("R2_BUCKET", "manga"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			PublicBaseURL:   os.Getenv("R2_PUBLIC_BASE_URL"),
		},
		KV: KVConfig{
			AccountID:   os.Getenv("CF_ACCOUNT_ID"),
			NamespaceID: os.Getenv("KV_NAMESPACE_ID"),
			APIToken:    os.Getenv("CF_API_TOKEN"),
		},
		OIDC: OIDCConfig{
			Issuer:          env("OIDC_ISSUER", env("PUBLIC_BASE_URL", "http://localhost:8080")),
			CryptoKey:       env("OIDC_CRYPTO_KEY", "dev-insecure-32-byte-crypto-key!"),
			RequiredScope:   env("OIDC_REQUIRED_SCOPE", "manga.write"),
			AccessTokenTTL:  time.Duration(envInt("OIDC_ACCESS_TTL_SEC", 3600)) * time.Second,
			RefreshTokenTTL: time.Duration(envInt("OIDC_REFRESH_TTL_SEC", 2592000)) * time.Second,
			AuthCodeTTL:     time.Duration(envInt("OIDC_AUTH_CODE_TTL_SEC", 300)) * time.Second,
			AdminEmail:      env("OIDC_ADMIN_EMAIL", "admin@example.com"),
			AdminPassword:   os.Getenv("OIDC_ADMIN_PASSWORD"),
			AdminRedirectURIs: splitCSV(env("ADMIN_REDIRECT_URIS",
				"http://localhost:3000/auth/callback")),
		},
		Image: ImageConfig{
			Quality:        envInt("AVIF_QUALITY", 55),
			Speed:          envInt("AVIF_SPEED", 6),
			MaxEdge:        envInt("AVIF_MAX_EDGE", 2048),
			ConvertTimeout: time.Duration(envInt("CONVERT_TIMEOUT_SEC", 600)) * time.Second,
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	// D1/R2 creds are only required at call time, so we warn rather than fail
	// hard here — this keeps `go run` usable for docs/health during dev.
	if c.Image.Quality < 0 || c.Image.Quality > 100 {
		return fmt.Errorf("AVIF_QUALITY must be 0..100, got %d", c.Image.Quality)
	}
	if c.Image.Speed < 0 || c.Image.Speed > 10 {
		return fmt.Errorf("AVIF_SPEED must be 0..10, got %d", c.Image.Speed)
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// splitCSV splits a comma-separated list, trimming whitespace and dropping
// empty entries.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
