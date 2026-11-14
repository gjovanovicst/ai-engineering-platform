package runs

import (
	"archive/tar"
	"bytes"
	"testing"
)

func tarFilesForMap(files map[string]string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	return buf.Bytes()
}

func TestRepoMapRanksMeetingFiles(t *testing.T) {
	archive := tarFilesForMap(map[string]string{
		"pagination.go":      "package main func paginate() {}",
		"pagination_test.go": "package main TestPagination",
		"README.md":          "pagination docs",
	})
	files := RepoMap(archive, "Fix pagination pagination", 10)
	found := false
	for _, file := range files {
		if file == "pagination.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("map = %v", files)
	}
}
