package runs

import (
	"archive/tar"
	"bytes"
	"testing"
)

func tarWith(files map[string]string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	return buf.Bytes()
}

func TestApplyTestFilesKeepsBaseImplementation(t *testing.T) {
	base := tarWith(map[string]string{"main.go": "base", "main_test.go": "old"})
	head := tarWith(map[string]string{"main.go": "head", "main_test.go": "new"})
	out, err := ApplyTestFiles(base, head, []string{"main_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	files, err := readTar(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(files["main.go"]) != "base" {
		t.Fatalf("implementation was overwritten: %q", files["main.go"])
	}
	if string(files["main_test.go"]) != "new" {
		t.Fatalf("test file = %q", files["main_test.go"])
	}
}
