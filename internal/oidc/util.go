package oidc

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// Scopes supported by this provider.
const (
	// ScopeMangaWrite gates ingest/convert routes.
	ScopeMangaWrite = "manga.write"
	// ScopeMangaRead gates reader routes that hand out presigned R2 page URLs.
	ScopeMangaRead = "manga.read"
	// ScopeConnectionsWrite gates the external-source OAuth connection routes
	// (/v1/connections) independently of the catalog write scope.
	ScopeConnectionsWrite = "connections.write"
	// ScopeUsersRead gates reading the admin user directory (/v1/users GET).
	ScopeUsersRead = "users.read"
	// ScopeUsersWrite gates creating/updating/deleting admin users.
	ScopeUsersWrite = "users.write"
)

// TaxonomyWriteKinds are the URL :kind path segments (plural) that each carry
// their own taxonomy write scope (taksonomi.<kind>.write). Taxonomy reads are
// gated by ScopeMangaRead instead, so they are not listed here.
var TaxonomyWriteKinds = []string{"genres", "categories", "authors", "artists"}

// ScopeTaxonomyWrite returns the per-kind taxonomy write scope for a URL :kind
// segment, e.g. "genres" -> "taksonomi.genres.write".
func ScopeTaxonomyWrite(kind string) string { return "taksonomi." + kind + ".write" }

// isTaxonomyWriteKind reports whether kind is a known taxonomy URL segment.
func isTaxonomyWriteKind(kind string) bool {
	return slices.Contains(TaxonomyWriteKinds, kind)
}

// --- D1 JSON value helpers (values arrive as string / float64 / nil) ---

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func floatVal(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func intVal(v any) int { return int(floatVal(v)) }

// timeVal parses an integer unix-seconds column into a time.Time.
func timeVal(v any) time.Time {
	switch t := v.(type) {
	case float64:
		if t == 0 {
			return time.Time{}
		}
		return time.Unix(int64(t), 0).UTC()
	case string:
		if t == "" {
			return time.Time{}
		}
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// jsonToStrings decodes a JSON-array text column into a []string.
func jsonToStrings(v any) []string {
	s := strVal(v)
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// stringsToJSON encodes a []string as a JSON-array text column value.
func stringsToJSON(vals []string) string {
	if vals == nil {
		vals = []string{}
	}
	b, _ := json.Marshal(vals)
	return string(b)
}

// --- argon2id password hashing (PHC string format) ---

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// hashPassword returns an argon2id PHC-formatted hash of the password.
func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// verifyPassword compares a password to an argon2id PHC hash in constant time.
func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=..,t=..,p=..", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	var mem uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, mem, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// randToken returns a URL-safe random token of n bytes of entropy.
func randToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail; fall back to a timestamp-derived value.
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
