package policydiff

import "testing"

func TestViolations(t *testing.T) {
	diff := "--- a/secrets/key\n+++ b/secrets/key\n"
	if got := Violations(diff, 0, []string{"secrets"}); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if got := Violations(diff, 0, []string{"src"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := Violations(diff, 5, nil); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}
