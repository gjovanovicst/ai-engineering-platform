package main

import (
	"context"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

// sandboxProofVerifier runs the acceptance tests against a base archive in an
// isolated sandbox. A non-zero exit is the expected outcome for a test that
// proves the change; zero means the test does not depend on the implementation.
type sandboxProofVerifier struct {
	sandbox sandbox.Sandbox
	image   string
}

func (v *sandboxProofVerifier) VerifyBase(ctx context.Context, archive []byte, _ []string, command string) (int, string, error) {
	return v.runArchive(ctx, archive, command)
}

// RunHead runs a command in the head workspace (e2e tests).
func (v *sandboxProofVerifier) RunHead(ctx context.Context, archive []byte, command string) (int, string, error) {
	return v.runArchive(ctx, archive, command)
}

func (v *sandboxProofVerifier) runArchive(ctx context.Context, archive []byte, command string) (int, string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	handle, err := v.sandbox.Create(runCtx, sandbox.Spec{
		Image: v.image, Workdir: "/workspace", NetworkDisabled: true,
		Env: map[string]string{"PATH": "/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"},
		CPU: 2, MemoryMB: 4096, Timeout: 10 * time.Minute,
	})
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = v.sandbox.Destroy(context.WithoutCancel(ctx), handle) }()
	if err := v.sandbox.Upload(runCtx, handle, archive); err != nil {
		return 0, "", err
	}
	// Login shells reset PATH, so make the toolchain explicit for both the e2e
	// command and the base-SHA replay.
	wrapped := "export PATH=/usr/local/go/bin:$PATH; " + command
	res, err := v.sandbox.Exec(runCtx, handle, []string{"sh", "-lc", wrapped}, nil)
	if err != nil {
		return 0, "", err
	}
	return res.ExitCode, strings.TrimSpace(res.Stdout + res.Stderr), nil
}
