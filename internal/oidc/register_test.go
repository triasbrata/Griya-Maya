package oidc

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRegisterUIRenders guards the invite-gated signup templates: the login card
// exposes a Register link, the register page renders its form, and a successful
// signup shows the awaiting-approval card. Pure render (no D1), so it stays green
// without storage wiring.
func TestRegisterUIRenders(t *testing.T) {
	// Login card carries the link into the register flow.
	w := httptest.NewRecorder()
	renderCard(w, loginData{ID: "auth-123"})
	if !strings.Contains(w.Body.String(), "/register?authRequestID=auth-123") {
		t.Fatalf("login card missing register link")
	}

	// Register page renders the (optional) invite form and a back-to-sign-in link.
	w = httptest.NewRecorder()
	renderPage(w, loginData{ID: "auth-123", Register: true})
	for _, want := range []string{
		`hx-post="/register"`,
		`name="code"`,
		"(optional)",
		"Create an account",
		"/login/username?authRequestID=auth-123",
	} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("register page missing %q", want)
		}
	}

	// Unverified success (no invite): account awaits approval.
	w = httptest.NewRecorder()
	renderCard(w, loginData{ID: "auth-123", Email: "a@b.com", Success: true})
	if body := w.Body.String(); !strings.Contains(body, "awaiting approval") || !strings.Contains(body, "a@b.com") {
		t.Fatalf("unverified success card wrong: %s", body)
	}

	// Verified success (valid invite): account is ready to sign in.
	w = httptest.NewRecorder()
	renderCard(w, loginData{ID: "auth-123", Email: "a@b.com", Success: true, Verified: true})
	if body := w.Body.String(); !strings.Contains(body, "You're all set") || !strings.Contains(body, "sign in now") {
		t.Fatalf("verified success card wrong: %s", body)
	}
}

func TestValidateInvite(t *testing.T) {
	const now = 1_000

	tests := []struct {
		name    string
		inv     *inviteRow
		email   string
		wantErr bool
	}{
		{
			name:  "open invite, unused, no expiry",
			inv:   &inviteRow{Code: "c"},
			email: "a@b.com",
		},
		{
			name:  "email-bound invite matches (case-insensitive)",
			inv:   &inviteRow{Code: "c", Email: "User@Example.com"},
			email: "user@example.com",
		},
		{
			name:  "not-yet-expired invite",
			inv:   &inviteRow{Code: "c", ExpiresAt: now + 1},
			email: "a@b.com",
		},
		{
			name:    "nil invite (unknown code)",
			inv:     nil,
			email:   "a@b.com",
			wantErr: true,
		},
		{
			name:    "already used",
			inv:     &inviteRow{Code: "c", UsedAt: now - 1},
			email:   "a@b.com",
			wantErr: true,
		},
		{
			name:    "expired",
			inv:     &inviteRow{Code: "c", ExpiresAt: now - 1},
			email:   "a@b.com",
			wantErr: true,
		},
		{
			name:    "email mismatch",
			inv:     &inviteRow{Code: "c", Email: "someone@else.com"},
			email:   "a@b.com",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInvite(tc.inv, tc.email, now)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
