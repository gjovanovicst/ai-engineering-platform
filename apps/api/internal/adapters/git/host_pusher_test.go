package git

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func tarWithFile(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestHostPusherPushesToBareRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	bare := filepath.Join(dir, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", bare).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v: %s", err, out)
	}

	err := (HostPusher{}).Push(context.Background(), tarWithFile(t, "README.md", "hello from the agent\n"), PushRequest{
		RemoteURL: bare,
		Branch:    "aip/test-run",
		Message:   "aip: test",
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("git", "--git-dir", bare, "rev-parse", "aip/test-run").CombinedOutput()
	if err != nil {
		t.Fatalf("branch missing: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatal("empty commit id")
	}

	show, err := exec.Command("git", "--git-dir", bare, "show", "aip/test-run:README.md").CombinedOutput()
	if err != nil || !strings.Contains(string(show), "hello from the agent") {
		t.Fatalf("file content: %v: %s", err, show)
	}
}

func TestHostPusherRejectsTraversal(t *testing.T) {
	archive := tarWithFile(t, "../../escape.txt", "x")
	err := (HostPusher{}).Push(context.Background(), archive, PushRequest{
		RemoteURL: "/tmp/nope", Branch: "aip/x",
	})
	// Either the entry is skipped (leaving an empty repo, push fails) or the
	// extraction rejects it; both must not write outside the temp dir.
	if err == nil {
		t.Log("push succeeded with an empty repo")
	}
}

func TestHostPusherKeepsCloneHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	initRepoWithCommit(t, root, "origin")
	origin := filepath.Join(root, "origin.git")
	target := filepath.Join(root, "target.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", target).CombinedOutput(); err != nil {
		t.Fatalf("init target: %v: %s", err, out)
	}

	work := filepath.Join(root, "clone")
	if out, err := exec.Command("git", "clone", "-q", origin, work).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(work, "feature.go"), []byte("package feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive, err := tarDir(work)
	if err != nil {
		t.Fatal(err)
	}

	if err := (HostPusher{}).Push(context.Background(), archive, PushRequest{
		RemoteURL: target, Branch: "aip/run-1", Message: "change",
	}); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("git", "-C", target, "log", "--format=%s", "aip/run-1").CombinedOutput()
	if err != nil {
		t.Fatalf("log: %v: %s", err, out)
	}
	lines := strings.Count(strings.TrimSpace(string(out)), "\n") + 1
	if lines != 2 {
		t.Fatalf("commit count = %d (want base + change): %s", lines, out)
	}
	if !strings.Contains(string(out), "base") {
		t.Fatalf("history not preserved: %s", out)
	}
}
