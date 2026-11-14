package git

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HostPusher extracts a workspace archive to a temporary directory and pushes
// it with the host's git binary. The sandbox never needs network access and the
// token never enters the sandbox.
type HostPusher struct {
	GitBin string
}

// Push implements Pusher for an exported workspace.
func (p HostPusher) Push(ctx context.Context, archive []byte, req PushRequest) error {
	if req.RemoteURL == "" || req.Branch == "" {
		return fmt.Errorf("git: push needs a remote url and a branch")
	}
	if strings.ContainsAny(req.Branch, " \t\n;|&$`") {
		return fmt.Errorf("git: invalid branch name")
	}

	dir, err := os.MkdirTemp("", "aip-push-*")
	if err != nil {
		return fmt.Errorf("git: temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	if err := extractTar(archive, dir); err != nil {
		return err
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		message = "aip: agent changes"
	}
	steps := [][]string{
		{"add", "-A"},
		{"-c", "user.email=aip@local", "-c", "user.name=aip", "commit", "-q", "-m", message},
		{"push", "-q", req.RemoteURL, "HEAD:refs/heads/" + req.Branch},
	}
	// A cloned repository already has .git; an empty workspace needs a new one.
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		//nolint:gosec // running git is the purpose; arguments are constant
		initCmd := exec.CommandContext(ctx, p.bin(), "init", "-q")
		initCmd.Dir = dir
		if out, err := initCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git: init: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}

	for _, args := range steps {
		//nolint:gosec // running git is the purpose; arguments are passed as argv
		cmd := exec.CommandContext(ctx, p.bin(), args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/true")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git: %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
		}
	}
	return nil
}

func (p HostPusher) bin() string {
	if p.GitBin == "" {
		return "git"
	}
	return p.GitBin
}

// extractTar writes an archive under dir, rejecting path traversal entries.
func extractTar(archive []byte, dir string) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("git: read archive: %w", err)
		}
		clean := filepath.Clean(header.Name)
		if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}
		target := filepath.Join(dir, clean)
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) && target != filepath.Clean(dir) {
			return fmt.Errorf("git: archive entry escapes the temp dir: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return err
			}
			//nolint:gosec // target is validated to stay inside the temp dir
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return err
			}
			//nolint:gosec // archive size is bounded by the sandbox export
			if _, err := io.Copy(file, reader); err != nil {
				_ = file.Close()
				return err
			}
			_ = file.Close()
		}
	}
}
