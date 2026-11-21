package git

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fakePnpm(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pnpm")
	// Write to a sibling and rename into place: exec'ing a file that is still
	// open for writing fails with ETXTBSY ("text file busy") on Linux, which
	// made this test flaky under -race.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPrepareNodeModulesInstallsFromTheLockfile(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"name":"demo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := fakePnpm(t, "mkdir -p node_modules/.bin\necho installed > node_modules/.bin/demo\n")
	ok, err := prepareNodeModules(context.Background(), repo, bin, 30*time.Second)
	if err != nil || !ok {
		t.Fatalf("prepareNodeModules = (%v, %v), want (true, nil)", ok, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "node_modules", ".bin", "demo")); err != nil {
		t.Fatalf("node_modules/.bin/demo: %v", err)
	}
}

func TestPrepareNodeModulesUsesTheAppWebLockfile(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	web := filepath.Join(repo, "apps", "web")
	if err := os.MkdirAll(web, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := fakePnpm(t, "mkdir -p node_modules\n")
	ok, err := prepareNodeModules(context.Background(), repo, bin, 30*time.Second)
	if err != nil || !ok {
		t.Fatalf("prepareNodeModules = (%v, %v)", ok, err)
	}
	if _, err := os.Stat(filepath.Join(web, "node_modules")); err != nil {
		t.Fatalf("apps/web/node_modules: %v", err)
	}
}

func TestPrepareNodeModulesWithoutALockfile(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ok, err := prepareNodeModules(context.Background(), repo, fakePnpm(t, "exit 0\n"), 5*time.Second)
	if err != nil || ok {
		t.Fatalf("prepareNodeModules = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestPrepareNodeModulesReportsAFailedInstall(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ok, err := prepareNodeModules(context.Background(), repo, fakePnpm(t, "echo boom >&2\nexit 1\n"), 10*time.Second)
	if err == nil || ok {
		t.Fatalf("prepareNodeModules = (%v, %v), want a failure", ok, err)
	}
}

func TestTarDirIncludesNodeModules(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "node_modules", ".bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "node_modules", ".bin", "demo"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, err := tarDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatal("node_modules missing from the archive")
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == "node_modules/.bin/demo" {
			return
		}
	}
}
