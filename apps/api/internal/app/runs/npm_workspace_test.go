package runs

import (
	"archive/tar"
	"bytes"
	"testing"
)

func testArchive(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range names {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWorkspaceHasNodeModules(t *testing.T) {
	t.Parallel()
	if workspaceHasNodeModules(testArchive(t, "main.go", "go.mod")) {
		t.Fatal("a Go workspace must not report node_modules")
	}
	if !workspaceHasNodeModules(testArchive(t, "package.json", "node_modules/.bin/demo")) {
		t.Fatal("a workspace with node_modules must be detected")
	}
	if !workspaceHasNodeModules(testArchive(t, "apps/web/node_modules/next/index.js")) {
		t.Fatal("a nested node_modules must be detected")
	}
}
