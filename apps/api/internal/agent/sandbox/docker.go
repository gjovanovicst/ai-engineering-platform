package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Docker runs one container per sandbox. It is the development and self-host
// implementation; Kubernetes replaces it in production without changing callers.
type Docker struct {
	// Binary defaults to "docker".
	Binary string
}

// NewDocker returns a Docker-backed sandbox.
func NewDocker() *Docker { return &Docker{Binary: "docker"} }

var _ Sandbox = (*Docker)(nil)

// Create starts a detached container and waits for it to be running.
func (d *Docker) Create(ctx context.Context, spec Spec) (Handle, error) {
	name := "aip-sbx-" + randomHex(6)
	workdir := spec.Workdir
	if workdir == "" {
		workdir = "/workspace"
	}

	args := []string{
		"run", "-d", "--rm", "--name", name,
		"--pids-limit", "512",
		"--user", "10001:10001",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--read-only",
		"--tmpfs", "/tmp:rw,exec,size=256m",
		"--tmpfs", workdir + ":rw,exec,size=512m,uid=10001,gid=10001",
		"--tmpfs", "/home/agent:rw,size=64m,uid=10001,gid=10001",
		"--tmpfs", "/cache:rw,size=512m,uid=10001,gid=10001",
		"-w", workdir,
	}
	if spec.NetworkDisabled {
		args = append(args, "--network", "none")
	}
	if spec.CPU > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", spec.CPU))
	}
	if spec.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", spec.MemoryMB))
	}
	for k, v := range spec.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, spec.Image, "sh", "-c", "sleep infinity")

	out, err := d.run(ctx, args...)
	if err != nil {
		return Handle{}, fmt.Errorf("sandbox: create %s: %w", spec.Image, err)
	}
	return Handle{ID: strings.TrimSpace(out)}, nil
}

// WriteFile copies content into the container without shell interpolation.
func (d *Docker) WriteFile(ctx context.Context, h Handle, path string, content []byte) error {
	//nolint:gosec // running commands is the sandbox's purpose; argv, fixed script, path as $1
	cmd := exec.CommandContext(ctx, d.bin(), "exec", "-i", h.ID, "sh", "-c",
		`mkdir -p "$(dirname "$1")" && cat > "$1"`, "sh", path)
	cmd.Stdin = bytes.NewReader(content)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sandbox: write %s: %w: %s", path, err, stderr.String())
	}
	return nil
}

// Upload extracts the archive into /workspace.
func (d *Docker) Upload(ctx context.Context, h Handle, archive []byte) error {
	//nolint:gosec // the sandbox's job is to run commands; argv only
	cmd := exec.CommandContext(ctx, d.bin(), "exec", "-i", h.ID, "tar", "-xf", "-", "-C", "/workspace")
	cmd.Stdin = bytes.NewReader(archive)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sandbox: upload: %w: %s", err, stderr.String())
	}
	return nil
}

type dockerProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

func (p *dockerProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *dockerProcess) Stdout() io.Reader     { return p.stdout }
func (p *dockerProcess) Stderr() io.Reader     { return p.stderr }
func (p *dockerProcess) Wait() error           { return p.cmd.Wait() }
func (p *dockerProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

// StartProcess runs an interactive command (MCP stdio) in the sandbox.
func (d *Docker) StartProcess(ctx context.Context, h Handle, cmdline []string, env map[string]string) (Process, error) {
	args := []string{"exec", "-i"}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "-e", key+"="+env[key])
	}
	args = append(args, h.ID)
	args = append(args, cmdline...)
	//nolint:gosec // exec is the sandbox's purpose; arguments are argv
	cmd := exec.CommandContext(ctx, d.bin(), args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("sandbox: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("sandbox: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("sandbox: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sandbox: start process: %w", err)
	}
	return &dockerProcess{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

// Exec runs a command, streaming stdout line by line.
func (d *Docker) Exec(ctx context.Context, h Handle, cmdline []string, onLine func(string) error) (Result, error) {
	args := append([]string{"exec", h.ID}, cmdline...)
	//nolint:gosec // exec is the sandbox's purpose; arguments are passed as argv
	cmd := exec.CommandContext(ctx, d.bin(), args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("sandbox: stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("sandbox: exec: %w", err)
	}

	var collected bytes.Buffer
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		collected.WriteString(line)
		collected.WriteByte('\n')
		if onLine != nil {
			if err := onLine(line); err != nil {
				_ = cmd.Process.Kill()
				return Result{}, err
			}
		}
	}

	waitErr := cmd.Wait()
	result := Result{Stdout: collected.String(), Stderr: stderr.String()}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(waitErr, &exitErr); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		return result, fmt.Errorf("sandbox: wait: %w", waitErr)
	}
	return result, nil
}

// Export streams a tar of /workspace out of the container.
func (d *Docker) Export(ctx context.Context, h Handle) ([]byte, error) {
	//nolint:gosec // export is the sandbox's purpose; arguments are passed as argv
	cmd := exec.CommandContext(ctx, d.bin(), "exec", h.ID, "tar", "-cf", "-", "-C", "/workspace", ".")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sandbox: export: %w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// Destroy removes the container. It is safe to call twice.
func (d *Docker) Destroy(ctx context.Context, h Handle) error {
	if h.ID == "" {
		return nil
	}
	// Use a fresh context: cancellation of the run must not leak the container.
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	_, err := d.run(cleanup, "rm", "-f", h.ID)
	if err != nil && !strings.Contains(err.Error(), "No such container") {
		return fmt.Errorf("sandbox: destroy %s: %w", h.ID, err)
	}
	return nil
}

func (d *Docker) bin() string {
	if d.Binary == "" {
		return "docker"
	}
	return d.Binary
}

func (d *Docker) run(ctx context.Context, args ...string) (string, error) {
	//nolint:gosec // exec is the sandbox's purpose; arguments are passed as argv
	cmd := exec.CommandContext(ctx, d.bin(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
