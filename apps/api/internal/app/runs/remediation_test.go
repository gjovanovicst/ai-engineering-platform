package runs

import "testing"

func TestRemediable(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"context_changed":        true,
		"dependency_missing":     true,
		"test_gate_failed":       false,
		"no_changes":             false,
		"workspace_clone_failed": false,
	}
	for code, want := range cases {
		if _, ok := Remediable(code); ok != want {
			t.Errorf("Remediable(%q) = %v, want %v", code, ok, want)
		}
	}
	if action, ok := Remediable("context_changed"); ok && action != "refreeze" {
		t.Errorf("action = %q, want refreeze", action)
	}
}

func TestDiffOnlyTests(t *testing.T) {
	t.Parallel()
	only := "diff --git a/src/a.test.ts b/src/a.test.ts\n+it('x', () => {})\n"
	if !diffOnlyTests(only) {
		t.Fatal("a test-only diff must be flagged")
	}
	mixed := only + "diff --git a/src/a.ts b/src/a.ts\n+export const x = 1\n"
	if diffOnlyTests(mixed) {
		t.Fatal("a diff with source must not be flagged")
	}
	if diffOnlyTests("") {
		t.Fatal("an empty diff is not a test-only change")
	}
}
