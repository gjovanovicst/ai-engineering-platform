package agent

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

// moduleCacheDir is where the sandbox keeps the Go module cache. It lives on
// the /cache tmpfs the sandbox driver already provides.
const moduleCacheDir = "/cache/gomod"

var (
	modulePathPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]*(?:/[A-Za-z0-9][A-Za-z0-9._~-]*)*$`)
	moduleVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
)

// validateModule keeps the host-side fetch to plain Go module coordinates: no
// flags, no URLs and no path traversal.
func validateModule(module, version string) error {
	if module == "" || version == "" {
		return fmt.Errorf("module and version are required")
	}
	if !modulePathPattern.MatchString(module) || strings.Contains(module, "..") {
		return fmt.Errorf("invalid module path %q", module)
	}
	if !moduleVersionPattern.MatchString(version) {
		return fmt.Errorf("invalid module version %q (want v1.2.3)", version)
	}
	return nil
}

// fetchModule downloads module@version on the host (which has network) and
// copies the resulting module cache into the offline sandbox. The sandbox
// itself never gets network access.
func fetchModule(ctx context.Context, sb sandbox.Sandbox, h sandbox.Handle, goBin, goproxy, module, version string) (string, error) {
	if err := validateModule(module, version); err != nil {
		return "", err
	}
	if goBin == "" {
		goBin = "go"
	}
	if goproxy == "" {
		goproxy = "https://proxy.golang.org,direct"
	}
	dir, err := os.MkdirTemp("", "aip-deps-")
	if err != nil {
		return "", fmt.Errorf("fetch module: temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	gomodcache := filepath.Join(dir, "mod")
	gomod := "module aip-fetch\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600); err != nil {
		return "", fmt.Errorf("fetch module: go.mod: %w", err)
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	//nolint:gosec // goBin comes from trusted config; module/version are validated above (no flags, no spaces).
	cmd := exec.CommandContext(fetchCtx, goBin, "mod", "download", "-json", module+"@"+version)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOMODCACHE="+gomodcache,
		"GOPATH="+filepath.Join(dir, "gopath"),
		"GOFLAGS=-mod=mod",
		"GOTOOLCHAIN=local",
		"GOPROXY="+goproxy,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("fetch %s@%s: %w: %s", module, version, err, truncateText(string(out), 600))
	}

	archive, size, err := tarModuleCache(gomodcache)
	if err != nil {
		return "", fmt.Errorf("fetch module: archive: %w", err)
	}
	if err := sb.WriteFile(ctx, h, "/tmp/aip-mod.tar", archive); err != nil {
		return "", fmt.Errorf("fetch module: copy archive: %w", err)
	}
	res, err := sb.Exec(ctx, h, []string{"sh", "-lc",
		"mkdir -p " + moduleCacheDir + " && chmod -R u+w " + moduleCacheDir + " 2>/dev/null; " +
			"tar -xf /tmp/aip-mod.tar -C " + moduleCacheDir + " && rm -f /tmp/aip-mod.tar"}, nil)
	if err != nil {
		return "", fmt.Errorf("fetch module: install: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("fetch module: install: exit %d: %s", res.ExitCode, truncateText(res.Stderr, 400))
	}
	return fmt.Sprintf("fetched %s@%s into the offline module cache (%d KiB); run go mod tidy and retry the tests",
		module, version, size/1024), nil
}

// tarModuleCache packs a module cache directory; the sandbox extracts it at
// moduleCacheDir.
func tarModuleCache(root string) ([]byte, int, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		hdr, herr := tar.FileInfoHeader(info, "")
		if herr != nil {
			return herr
		}
		hdr.Name = filepath.ToSlash(rel)
		// Go marks module files read-only; the sandbox must be able to write on
		// top of them (locks, retries), so normalise the modes.
		if d.IsDir() {
			hdr.Name += "/"
			hdr.Mode = 0o755
		} else {
			hdr.Mode = 0o644
		}
		if d.Type()&fs.ModeSymlink != 0 {
			link, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			hdr.Linkname = link
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		//nolint:gosec // p comes from walking the temp module cache we just created.
		data, derr := os.ReadFile(p)
		if derr != nil {
			return derr
		}
		_, werr := tw.Write(data)
		return werr
	})
	if err != nil {
		return nil, 0, err
	}
	if err := tw.Close(); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), buf.Len(), nil
}
