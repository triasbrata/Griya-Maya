package oidc

import "testing"

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
