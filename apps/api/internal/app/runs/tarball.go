package runs

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ApplyTestFiles copies the selected files from the head archive onto the base
// archive. The result is a repository with the original implementation and the
// new/changed tests, which is exactly the base-SHA negative guard input.
func ApplyTestFiles(base, head []byte, files []string) ([]byte, error) {
	want := map[string]bool{}
	for _, file := range files {
		want[strings.TrimPrefix(file, "./")] = true
	}
	headFiles, err := readTar(head)
	if err != nil {
		return nil, fmt.Errorf("read head archive: %w", err)
	}
	baseFiles, err := readTar(base)
	if err != nil {
		return nil, fmt.Errorf("read base archive: %w", err)
	}
	for file := range want {
		content, ok := headFiles[file]
		if !ok {
			return nil, fmt.Errorf("test file %s is not in the head archive", file)
		}
		baseFiles[file] = content
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range baseFiles {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(content); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readTar(archive []byte) (map[string][]byte, error) {
	out := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, 20<<20))
		if err != nil {
			return nil, err
		}
		out[strings.TrimPrefix(hdr.Name, "./")] = data
	}
	return out, nil
}
