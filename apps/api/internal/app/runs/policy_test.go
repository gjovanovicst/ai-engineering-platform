package runs

import "testing"

func TestPolicyViolation(t *testing.T) {
	diff := "--- a/secrets/key\n+++ b/secrets/key\n"
	if got := policyViolation(diff, 0, []string{"secrets"}); got == "" {
		t.Fatal("expected forbidden path violation")
	}
	if got := policyViolation(diff, 0, []string{"src"}); got != "" {
		t.Fatalf("unexpected violation: %q", got)
	}
	if got := policyViolation(diff, 5, nil); got == "" {
		t.Fatal("expected size violation")
	}
}
