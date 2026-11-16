package agent

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

func TestValidateModule(t *testing.T) {
	t.Parallel()
	valid := [][2]string{
		{"golang.org/x/crypto", "v0.36.0"},
		{"example.com/m", "v1.2.3-rc.1"},
		{"github.com/Azure/go-ansiterm", "v0.0.0-20210617225240-d185dfc1b5a1"},
	}
	for _, tc := range valid {
		if err := validateModule(tc[0], tc[1]); err != nil {
			t.Fatalf("validateModule(%q, %q) = %v, want nil", tc[0], tc[1], err)
		}
	}
	invalid := [][2]string{
		{"", "v1.0.0"},
		{"-flag", "v1.0.0"},
		{"a/../b", "v1.0.0"},
		{"a b", "v1.0.0"},
		{"golang.org/x/crypto", "main"},
		{"golang.org/x/crypto", "1.0.0"},
		{"golang.org/x/crypto", "v1.0.0; rm -rf /"},
	}
	for _, tc := range invalid {
		if err := validateModule(tc[0], tc[1]); err == nil {
			t.Fatalf("validateModule(%q, %q) = nil, want an error", tc[0], tc[1])
		}
	}
}

func TestFetchModuleInstallsTheModuleCache(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	script := filepath.Join(dir, "go-fake.sh")
	body := `#!/bin/sh
mkdir -p "$GOMODCACHE/cache/download/example.com/x/@v" "$GOMODCACHE/example.com/x@v1.0.0"
printf 'module example.com/x\n' > "$GOMODCACHE/example.com/x@v1.0.0/go.mod"
printf 'package x\n' > "$GOMODCACHE/example.com/x@v1.0.0/x.go"
chmod 444 "$GOMODCACHE/example.com/x@v1.0.0/x.go"
printf '{"Path":"example.com/x","Version":"v1.0.0"}\n'
`
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	sb := &toolSandbox{}
	out, err := fetchModule(context.Background(), sb, sandbox.Handle{ID: "h"}, script, "off", "example.com/x", "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fetched example.com/x@v1.0.0") {
		t.Fatalf("output = %q", out)
	}
	raw, ok := sb.written["/tmp/aip-mod.tar"]
	if !ok {
		t.Fatalf("archive not written, got %v", sb.written)
	}
	names := tarNames(t, []byte(raw))
	if !names["example.com/x@v1.0.0/x.go"] || !names["example.com/x@v1.0.0/go.mod"] {
		t.Fatalf("archive misses the module: %v", names)
	}
	if !strings.Contains(strings.Join(flatten(sb.execs), " "), "tar -xf /tmp/aip-mod.tar -C /cache/gomod") {
		t.Fatalf("archive was not extracted into the module cache: %v", sb.execs)
	}
	if mode := tarMode(t, []byte(raw), "example.com/x@v1.0.0/x.go"); mode != 0o644 {
		t.Fatalf("file mode = %o, want 644 (writable in the sandbox)", mode)
	}
	if mode := tarMode(t, []byte(raw), "example.com/x@v1.0.0/"); mode != 0o755 {
		t.Fatalf("dir mode = %o, want 755", mode)
	}
}

func TestFetchModuleRejectsBadCoordinatesBeforeRunningGo(t *testing.T) {
	t.Parallel()
	sb := &toolSandbox{}
	if _, err := fetchModule(context.Background(), sb, sandbox.Handle{}, "go", "off", "evil; rm -rf /", "v1.0.0"); err == nil {
		t.Fatal("want an error")
	}
	if len(sb.written) != 0 || len(sb.execs) != 0 {
		t.Fatalf("invalid coordinates reached the sandbox: %v %v", sb.written, sb.execs)
	}
}

func tarNames(t *testing.T, archive []byte) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		names[hdr.Name] = true
	}
}

func tarMode(t *testing.T, archive []byte, name string) int64 {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("%s not found in the archive", name)
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == name {
			return hdr.Mode
		}
	}
}

func flatten(cmds [][]string) []string {
	var out []string
	for _, cmd := range cmds {
		out = append(out, strings.Join(cmd, " "))
	}
	return out
}
