package git

import (
	"strings"
	"testing"
)

func TestRedactCredentials(t *testing.T) {
	t.Parallel()
	const remote = "https://x-access-token:github_pat_secret123@github.com/acme/repo.git"
	message := "git: push: exit status 1: To " + remote + "\n ! [remote rejected] HEAD -> aip/1"
	got := redactCredentials(message, remote)
	if strings.Contains(got, "github_pat_secret123") || strings.Contains(got, "x-access-token:") {
		t.Fatalf("credentials leaked: %s", got)
	}
	if !strings.Contains(got, "***@github.com/acme/repo.git") {
		t.Fatalf("redacted message = %s", got)
	}
}

func TestRedactCredentialsWithoutUserinfo(t *testing.T) {
	t.Parallel()
	const remote = "https://github.com/acme/repo.git"
	message := "fatal: repository not found: " + remote
	if got := redactCredentials(message, remote); got != message {
		t.Fatalf("message changed unnecessarily: %s", got)
	}
}
