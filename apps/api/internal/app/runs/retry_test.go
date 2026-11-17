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

func TestClassifyTestFailureDetectsMissingDependencies(t *testing.T) {
	t.Parallel()
	cases := []string{
		"main.go:5:2: no required module provides package golang.org/x/crypto/argon2",
		"missing go.sum entry for module providing package golang.org/x/crypto/argon2",
		"cannot find module providing package example.com/missing",
		"go: module lookup disabled by GOPROXY=off",
		"npm ERR! code ENOTFOUND\nnpm ERR! network request to https://registry.npmjs.org failed",
	}
	for _, output := range cases {
		if got := classifyTestFailure(output); got != "dependency" {
			t.Fatalf("classify(%q) = %q, want dependency", output, got)
		}
	}
}
