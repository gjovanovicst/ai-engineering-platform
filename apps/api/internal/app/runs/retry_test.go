package runs

import "testing"

func TestClassifyTestFailure(t *testing.T) {
	if got := classifyTestFailure("dial tcp: connection refused"); got != "infra" {
		t.Fatalf("got %q", got)
	}
	if got := classifyTestFailure("WARNING: flaky test retried"); got != "flaky" {
		t.Fatalf("flaky: got %q", got)
	}
	if got := classifyTestFailure("--- FAIL: TestPager (0.00s)"); got != "code" {
		t.Fatalf("got %q", got)
	}
}
