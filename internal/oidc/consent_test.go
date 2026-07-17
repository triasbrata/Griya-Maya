package oidc

import (
	"reflect"
	"testing"
)

func TestMissingScopes(t *testing.T) {
	cases := []struct {
		name      string
		requested []string
		granted   []string
		want      []string
	}{
		{"first consent, nothing granted", []string{"openid", "manga.read"}, nil, []string{"openid", "manga.read"}},
		{"all already granted -> skip", []string{"openid", "manga.read"}, []string{"openid", "manga.read", "email"}, []string{}},
		{"incremental delta only", []string{"openid", "manga.read", "manga.write"}, []string{"openid", "manga.read"}, []string{"manga.write"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := missingScopes(c.requested, c.granted)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("missingScopes(%v,%v) = %v, want %v", c.requested, c.granted, got, c.want)
			}
		})
	}
}

func TestUnionScopes_DedupPreservesOrder(t *testing.T) {
	got := unionScopes([]string{"openid", "manga.read"}, []string{"manga.read", "manga.write"})
	want := []string{"openid", "manga.read", "manga.write"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unionScopes = %v, want %v", got, want)
	}
}

func TestScopeLabel_FallsBackToRaw(t *testing.T) {
	if got := scopeLabel("manga.read"); got != "Read your library and pages" {
		t.Fatalf("known scope label = %q", got)
	}
	if got := scopeLabel("custom.unknown"); got != "custom.unknown" {
		t.Fatalf("unknown scope should fall back to raw, got %q", got)
	}
}
