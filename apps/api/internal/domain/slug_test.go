package domain

import "testing"

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Acme":            "acme",
		"  Acme   Inc.  ": "acme-inc",
		"Đorđe & Sons":    "đorđe-sons",
		"ab":              "ab-org",
		"!!!":             "-org",
		"a-b_c.d":         "a-b-c-d",
	}
	for in, want := range tests {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUniqueSlug(t *testing.T) {
	taken := map[string]bool{"acme": true, "acme-2": true}
	got := UniqueSlug("acme", func(s string) bool { return taken[s] })
	if got != "acme-3" {
		t.Fatalf("UniqueSlug = %q, want acme-3", got)
	}
}
