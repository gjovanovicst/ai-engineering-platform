//go:build integration

package sandbox_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

func TestDockerSandboxLifecycle(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is not installed")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	sb := sandbox.NewDocker()
	handle, err := sb.Create(ctx, sandbox.Spec{
		Image:           "alpine:3.20",
		Workdir:         "/workspace",
		NetworkDisabled: true,
		CPU:             1,
		MemoryMB:        128,
		Timeout:         time.Minute,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		if err := sb.Destroy(context.Background(), handle); err != nil {
			t.Errorf("destroy: %v", err)
		}
	})

	if err := sb.WriteFile(ctx, handle, "/workspace/nested/hello.txt", []byte("from the host\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	var lines []string
	res, err := sb.Exec(ctx, handle, []string{"sh", "-lc", "cat /workspace/nested/hello.txt && echo second"}, func(line string) error {
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", res.ExitCode, res.Stderr)
	}
	if len(lines) != 2 || lines[0] != "from the host" {
		t.Fatalf("lines = %v", lines)
	}
	if !strings.Contains(res.Stdout, "second") {
		t.Fatalf("stdout = %q", res.Stdout)
	}

	// A non-zero exit is reported, not treated as an infrastructure error.
	res, err = sb.Exec(ctx, handle, []string{"sh", "-lc", "exit 3"}, nil)
	if err != nil {
		t.Fatalf("exec exit: %v", err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("exit = %d, want 3", res.ExitCode)
	}

	if err := sb.Destroy(ctx, handle); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if err := sb.Destroy(ctx, handle); err != nil {
		t.Fatalf("second destroy must be idempotent: %v", err)
	}
}
