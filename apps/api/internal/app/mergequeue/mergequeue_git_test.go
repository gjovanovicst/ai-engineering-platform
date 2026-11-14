package mergequeue

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type gitQueueMerger struct {
	dirs     map[int]string
	branches map[int]string
	merged   []int
	rebased  []int
}

func (g *gitQueueMerger) Rebase(_ context.Context, _, _ string, number int) error {
	dir, branch := g.dirs[number], g.branches[number]
	if _, err := gitRun(dir, "fetch", "origin"); err != nil {
		return err
	}
	if _, err := gitRun(dir, "checkout", branch); err != nil {
		return err
	}
	_, err := gitRun(dir, "rebase", "origin/main")
	if err == nil {
		g.rebased = append(g.rebased, number)
	}
	return err
}

func (g *gitQueueMerger) Merge(_ context.Context, _, _ string, number int) error {
	dir, branch := g.dirs[number], g.branches[number]
	if _, err := gitRun(dir, "checkout", "main"); err != nil {
		return err
	}
	if _, err := gitRun(dir, "reset", "--hard", "origin/main"); err != nil {
		return err
	}
	if _, err := gitRun(dir, "merge", "--no-ff", "--no-edit", branch); err != nil {
		return err
	}
	if _, err := gitRun(dir, "push", "origin", "main"); err != nil {
		return err
	}
	g.merged = append(g.merged, number)
	return nil
}

func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=queue-test", "GIT_AUTHOR_EMAIL=queue@test.invalid",
		"GIT_COMMITTER_NAME=queue-test", "GIT_COMMITTER_EMAIL=queue@test.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitRun(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// TestProcessAllHandlesRealGitRebaseConflict proves the queue with real Git:
// PR 1 lands, PR 2 then conflicts on the same line and must not be merged.
func TestProcessAllHandlesRealGitRebaseConflict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	mustGit(t, root, "init", "--bare", bare)

	seed := filepath.Join(root, "seed")
	mustGit(t, root, "clone", bare, seed)
	mustGit(t, seed, "config", "user.email", "queue@test.invalid")
	mustGit(t, seed, "config", "user.name", "queue-test")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, seed, "add", "README.md")
	mustGit(t, seed, "commit", "-m", "base")
	mustGit(t, seed, "branch", "-M", "main")
	mustGit(t, seed, "push", "-u", "origin", "main")
	mustGit(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")

	dirs := map[int]string{}
	branches := map[int]string{1: "pr-a", 2: "pr-b"}
	for number, branch := range branches {
		dir := filepath.Join(root, "pr", branch)
		mustGit(t, root, "clone", bare, dir)
		mustGit(t, dir, "config", "user.email", "queue@test.invalid")
		mustGit(t, dir, "config", "user.name", "queue-test")
		mustGit(t, dir, "checkout", "-b", branch)
		value := "A\n"
		if number == 2 {
			value = "B\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
		mustGit(t, dir, "commit", "-am", "change "+branch)
		mustGit(t, dir, "push", "origin", branch)
		dirs[number] = dir
	}

	store := &fakeStore{items: []Item{{Number: 1}, {Number: 2}}}
	merger := &gitQueueMerger{dirs: dirs, branches: branches}
	results, err := New(store, merger, nil).ProcessAll(context.Background(), "org", "project")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	if len(merger.merged) != 1 || merger.merged[0] != 1 {
		t.Fatalf("merged = %v, want [1]", merger.merged)
	}
	if len(merger.rebased) != 1 || merger.rebased[0] != 1 {
		t.Fatalf("rebased = %v, want [1]", merger.rebased)
	}
	if results[1].Err == nil || !strings.Contains(results[1].Err.Error(), "CONFLICT") {
		t.Fatalf("expected real conflict for PR 2, got %v", results[1].Err)
	}
	if len(store.status) != 2 || store.status[0] != "merged" || store.status[1] != "failed" {
		t.Fatalf("status = %v", store.status)
	}
	if len(store.details) != 2 || !strings.Contains(store.details[1], "rebase:") {
		t.Fatalf("details = %v", store.details)
	}
}
