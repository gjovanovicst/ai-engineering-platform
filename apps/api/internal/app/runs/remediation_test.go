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
