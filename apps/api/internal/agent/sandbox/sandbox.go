// Package sandbox isolates agent execution. Every run gets its own instance;
// it is destroyed when the run ends, regardless of outcome.
package sandbox

import (
	"context"
	"io"
	"time"
)

// Spec describes one ephemeral sandbox.
type Spec struct {
	Image           string
	Workdir         string
	Env             map[string]string
	CPU             float64
	MemoryMB        int
	Timeout         time.Duration
	NetworkDisabled bool
}

// Handle identifies a running sandbox.
type Handle struct {
	ID string
}

// Result is the outcome of one command.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Process is a long-running command inside a sandbox (used by MCP servers).
type Process interface {
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
	Wait() error
	Kill() error
}

// Sandbox is the port implemented by Docker (dev) and Kubernetes (prod).
type Sandbox interface {
	Create(ctx context.Context, spec Spec) (Handle, error)
	// Upload extracts a tar archive into the sandbox workdir (repository clone).
	Upload(ctx context.Context, h Handle, archive []byte) error
	// WriteFile creates parent directories and writes the file in the sandbox.
	WriteFile(ctx context.Context, h Handle, path string, content []byte) error
	// Exec runs a command and streams stdout lines to onLine (when not nil).
	Exec(ctx context.Context, h Handle, cmd []string, onLine func(string) error) (Result, error)
	// StartProcess starts an interactive process (stdin/stdout), used by MCP.
	StartProcess(ctx context.Context, h Handle, cmd []string, env map[string]string) (Process, error)
	// Export returns the workspace as a tar archive (used for pushing from the
	// host so the sandbox itself never needs network access).
	Export(ctx context.Context, h Handle) ([]byte, error)
	// Destroy is idempotent: calling it twice must not fail.
	Destroy(ctx context.Context, h Handle) error
}
