package oidc

import (
	"context"
	"time"
)

// consentScopes returns the set of scopes the user has already granted the
// given client (nil if the user has never consented for this client).
func (s *Storage) consentScopes(ctx context.Context, userID, clientID string) ([]string, error) {
	rows, err := s.d1.Query(ctx,
		`SELECT scopes FROM oidc_user_consent WHERE user_id = ?1 AND client_id = ?2`,
		userID, clientID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return jsonToStrings(rows[0]["scopes"]), nil
}

// saveConsent records (union-merges) the scopes the user approved for the
// client, so a later auth request for the same-or-subset scope set skips the
// consent screen.
func (s *Storage) saveConsent(ctx context.Context, userID, clientID string, scopes []string) error {
	existing, err := s.consentScopes(ctx, userID, clientID)
	if err != nil {
		return err
	}
	merged := unionScopes(existing, scopes)
	now := time.Now().Unix()
	return s.d1.Exec(ctx,
		`INSERT INTO oidc_user_consent (user_id, client_id, scopes, created_at, updated_at)
		 VALUES (?1, ?2, ?3, ?4, ?4)
		 ON CONFLICT(user_id, client_id) DO UPDATE SET scopes = ?3, updated_at = ?4`,
		userID, clientID, stringsToJSON(merged), now)
}

// missingScopes returns the requested scopes not present in granted, preserving
// request order. Empty result means every requested scope is already consented.
func missingScopes(requested, granted []string) []string {
	have := make(map[string]struct{}, len(granted))
	for _, s := range granted {
		have[s] = struct{}{}
	}
	out := make([]string, 0, len(requested))
	for _, s := range requested {
		if _, ok := have[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}

// unionScopes merges two scope lists, de-duplicating while preserving first-seen
// order (existing grants first, then any newly approved scopes).
func unionScopes(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, s := range list {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// scopeLabels maps raw OAuth scopes to human-readable consent descriptions.
// Unknown scopes fall back to their raw name (see scopeLabel).
var scopeLabels = map[string]string{
	"openid":                         "Verify your identity",
	"profile":                        "Access your basic profile",
	"email":                          "Access your email address",
	"offline_access":                 "Stay signed in when you're away",
	ScopeMangaRead:                   "Read your library and pages",
	ScopeMangaWrite:                  "Add and modify library content",
	ScopeConnectionsWrite:            "Manage external source connections",
	ScopeUsersRead:                   "View the user directory",
	ScopeUsersWrite:                  "Manage users",
	ScopeAdminRead:                   "View admin data",
	ScopeAdminWrite:                  "Manage admin settings",
	ScopeTaxonomyWrite("genres"):     "Manage genres",
	ScopeTaxonomyWrite("authors"):    "Manage authors",
	ScopeTaxonomyWrite("artists"):    "Manage artists",
	ScopeTaxonomyWrite("categories"): "Manage categories",
}

// scopeLabel returns a friendly description for a scope, falling back to the raw
// scope string for anything not in scopeLabels (e.g. dynamically registered).
func scopeLabel(scope string) string {
	if label, ok := scopeLabels[scope]; ok {
		return label
	}
	return scope
}

// scopeView is a scope paired with its human-readable label, for the consent template.
type scopeView struct {
	Name  string
	Label string
}

// scopeViews decorates raw scopes with their friendly labels.
func scopeViews(scopes []string) []scopeView {
	out := make([]scopeView, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, scopeView{Name: s, Label: scopeLabel(s)})
	}
	return out
}
