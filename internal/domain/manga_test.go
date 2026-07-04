package domain

import "testing"

func TestCatalogFilter_SortColumn(t *testing.T) {
	cases := []struct {
		name        string
		sort        string
		feedDefault string
		want        string
	}{
		{"empty falls back to feed default", "", "popular", "popularity"},
		{"empty latest default", "", "updated", "updated_at"},
		{"title", "title", "popular", "title"},
		{"latest", "latest", "popular", "updated_at"},
		{"updated", "updated", "popular", "updated_at"},
		{"rating maps to popularity", "rating", "updated", "popularity"},
		{"popular maps to popularity", "popular", "updated", "popularity"},
		{"unknown falls back to updated_at", "bogus", "popular", "updated_at"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := CatalogFilter{Sort: tc.sort}
			if got := f.SortColumn(tc.feedDefault); got != tc.want {
				t.Errorf("SortColumn(%q) with sort=%q = %q, want %q", tc.feedDefault, tc.sort, got, tc.want)
			}
		})
	}
}
