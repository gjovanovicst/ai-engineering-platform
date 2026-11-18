package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// prepareNodeModules installs the web dependencies on the host, from the
// repository lockfile, before the workspace is archived for the offline
// sandbox. It looks for pnpm-lock.yaml at the repository root or in apps/web
// (the platform layout) and runs pnpm in that directory.
//
// The install is best-effort: the caller logs a failure and the run continues
// without node_modules. It returns whether a lockfile was found and the
// install succeeded.
func prepareNodeModules(ctx context.Context, dir, pnpmBin string, timeout time.Duration) (bool, error) {
	if pnpmBin == "" {
		pnpmBin = "pnpm"
	}
	lockDir := ""
	for _, candidate := range []string{dir, filepath.Join(dir, "apps", "web")} {
		if _, err := os.Stat(filepath.Join(candidate, "pnpm-lock.yaml")); err == nil {
			lockDir = candidate
			break
		}
	}
	if lockDir == "" {
		return false, nil
	}
	installCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	//nolint:gosec // pnpmBin comes from trusted config; the arguments are fixed.
	cmd := exec.CommandContext(installCtx, pnpmBin, "install", "--frozen-lockfile", "--ignore-scripts")
	cmd.Dir = lockDir
	cmd.Env = append(os.Environ(), "CI=true")
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("pnpm install in %s: %w: %s", lockDir, err, truncateOutput(string(out)))
	}
	return true, nil
}

// truncateOutput keeps the tail of an install log short enough for an event.
func truncateOutput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 600 {
		s = s[len(s)-600:]
	}
	return s
}
