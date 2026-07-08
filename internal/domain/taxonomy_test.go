package domain

import "testing"

func TestTaxonomyKind_Valid(t *testing.T) {
	valid := []TaxonomyKind{TaxonomyGenre, TaxonomyAuthor, TaxonomyArtist}
	for _, k := range valid {
		if !k.Valid() {
			t.Errorf("%q should be valid", k)
		}
	}
	// category was retired in favor of the re-introduced genre taxonomy.
	if TaxonomyKind("category").Valid() {
		t.Error("category should be invalid (retired)")
	}
	if TaxonomyKind("bogus").Valid() {
		t.Error("bogus should be invalid")
	}
}

func TestTaxonomyKind_HasSlug(t *testing.T) {
	if !TaxonomyGenre.HasSlug() {
		t.Error("genre should have a slug")
	}
	if TaxonomyAuthor.HasSlug() || TaxonomyArtist.HasSlug() {
		t.Error("author/artist should not have slugs")
	}
}
