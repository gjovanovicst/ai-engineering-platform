// Package monorepo detects affected packages from a diff and manifest files.
package monorepo

import (
	"archive/tar"
	"bytes"
	"io"
	"path"
	"sort"
	"strings"
)

var manifestNames = map[string]bool{"go.mod": true, "package.json": true, "pyproject.toml": true, "Cargo.toml": true}

// Manifests lists package roots (directories containing a manifest) in a tar.
func Manifests(archive []byte) []string {
	seen := map[string]bool{}
	var out []string
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF || err != nil {
			break
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if !manifestNames[path.Base(name)] {
			continue
		}
		dir := path.Dir(name)
		if dir == "" {
			dir = "."
		}
		if !seen[dir] {
			seen[dir] = true
			out = append(out, dir)
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// Affected returns package roots touched by the diff, using the longest matching
// manifest directory (repository root is ".").
func Affected(diff string, manifests []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, file := range changedFiles(diff) {
		pkg := packageFor(file, manifests)
		if seen[pkg] {
			continue
		}
		seen[pkg] = true
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}

func changedFiles(diff string) []string {
	var files []string
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			files = append(files, strings.TrimPrefix(line, "+++ b/"))
		case strings.HasPrefix(line, "--- a/"):
			files = append(files, strings.TrimPrefix(line, "--- a/"))
		}
	}
	return files
}

func packageFor(file string, manifests []string) string {
	dir := path.Dir(strings.TrimPrefix(file, "./"))
	for _, root := range manifests {
		if root == "." || dir == root || strings.HasPrefix(dir, root+"/") {
			return root
		}
	}
	return "."
}

// ManifestNames is exposed for documentation/tests.
func ManifestNames() []string {
	out := make([]string, 0, len(manifestNames))
	for name := range manifestNames {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
