package monorepo

import (
	"archive/tar"
	"bytes"
	"testing"
)

func manifestArchive(t *testing.T, paths ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, p := range paths {
		_ = tw.WriteHeader(&tar.Header{Name: p, Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte("x"))
	}
	_ = tw.Close()
	return buf.Bytes()
}

func TestAffectedWithManifests(t *testing.T) {
	manifests := Manifests(manifestArchive(t, "go.mod", "apps/api/go.mod", "packages/ui/package.json"))
	diff := "--- a/apps/api/internal/x.go\n+++ b/apps/api/internal/x.go\n--- a/packages/ui/button.tsx\n+++ b/packages/ui/button.tsx\n--- a/README.md\n+++ b/README.md\n"
	got := Affected(diff, manifests)
	if len(got) != 3 || got[0] != "." || got[1] != "apps/api" || got[2] != "packages/ui" {
		t.Fatalf("got = %v (manifests %v)", got, manifests)
	}
}
