package domain

import "testing"

func TestTaxonomyKind_Valid(t *testing.T) {
	valid := []TaxonomyKind{TaxonomyGenre, TaxonomyCategory, TaxonomyAuthor, TaxonomyArtist}
	for _, k := range valid {
		if !k.Valid() {
			t.Errorf("%q should be valid", k)
		}
	}
	if TaxonomyKind("bogus").Valid() {
		t.Error("bogus should be invalid")
	}
}

func TestTaxonomyKind_HasSlug(t *testing.T) {
	if !TaxonomyGenre.HasSlug() || !TaxonomyCategory.HasSlug() {
		t.Error("genre/category should have slugs")
	}
	if TaxonomyAuthor.HasSlug() || TaxonomyArtist.HasSlug() {
		t.Error("author/artist should not have slugs")
	}
}
