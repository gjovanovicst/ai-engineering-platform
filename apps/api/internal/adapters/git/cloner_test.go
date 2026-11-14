package git

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

func initRepoWithCommit(t *testing.T, root, name string) string {
	t.Helper()
	bare := filepath.Join(root, name+".git")
	if out, err := exec.Command("git", "init", "--bare", "-q", bare).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v: %s", err, out)
	}
	work := filepath.Join(root, "work")
	if out, err := exec.Command("git", "clone", "-q", bare, work).CombinedOutput(); err != nil {
		t.Fatalf("clone work: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-C", work, "config", "user.email", "t@example.com"},
		{"-C", work, "config", "user.name", "t"},
		{"-C", work, "add", "-A"},
		{"-C", work, "commit", "-q", "-m", "base"},
		{"-C", work, "push", "-q", "origin", "HEAD"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return bare
}

func TestClonerReturnsRepositoryArchive(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	bare := initRepoWithCommit(t, root, "group/project")
	// remoteURL builds base + "/" + name + ".git"; point the base at the temp dir.
	cloner := &Cloner{
		Repos:      fakeLookup{repo: domain.Repo{ID: "r", Provider: domain.ProviderGitHub, Name: "group/project", DefaultBranch: "master"}},
		CloneBases: map[string]string{"github": "file://" + root},
	}
	archive, err := cloner.Clone(context.Background(), "o", "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(archive) == 0 {
		t.Fatal("empty archive")
	}
	names := map[string]bool{}
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err != nil {
			break
		}
		names[header.Name] = true
	}
	if !names["README.md"] || !names[".git/HEAD"] && !names[".git/config"] {
		t.Fatalf("archive missing repo files: %v", names)
	}
	_ = bare
}

func TestClonerWithoutRepoReturnsNil(t *testing.T) {
	cloner := &Cloner{Repos: fakeLookup{err: domain.ErrNotFound}}
	archive, err := cloner.Clone(context.Background(), "o", "p")
	if err != nil || archive != nil {
		t.Fatalf("archive=%v err=%v", archive, err)
	}
}
