package git

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

type fakeSandbox struct {
	cmd   []string
	lines []string
	err   error
}

func (f *fakeSandbox) Create(context.Context, sandbox.Spec) (sandbox.Handle, error) {
	return sandbox.Handle{ID: "sbx"}, nil
}
func (f *fakeSandbox) Upload(context.Context, sandbox.Handle, []byte) error { return nil }

func (f *fakeSandbox) WriteFile(context.Context, sandbox.Handle, string, []byte) error { return nil }
func (f *fakeSandbox) Exec(_ context.Context, _ sandbox.Handle, cmd []string, _ func(string) error) (sandbox.Result, error) {
	f.cmd = cmd
	return sandbox.Result{ExitCode: 0, Stdout: strings.Join(f.lines, "\n")}, f.err
}
func (f *fakeSandbox) Export(context.Context, sandbox.Handle) ([]byte, error) {
	return nil, nil
}
func (f *fakeSandbox) Destroy(context.Context, sandbox.Handle) error { return nil }

func TestSandboxPusherBuildsScript(t *testing.T) {
	sb := &fakeSandbox{}
	err := SandboxPusher{}.Push(context.Background(), sb, sandbox.Handle{ID: "sbx"}, PushRequest{
		RemoteURL: "https://x-access-token:tok@github.com/o/r.git",
		Branch:    "aip/run-1",
		Message:   "aip: T-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	script := sb.cmd[2]
	for _, want := range []string{"git add -A", "git push", "aip/run-1", "refs/heads/aip/run-1"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q: %s", want, script)
		}
	}
}

func TestSandboxPusherRejectsUnsafeBranch(t *testing.T) {
	sb := &fakeSandbox{}
	err := SandboxPusher{}.Push(context.Background(), sb, sandbox.Handle{ID: "sbx"}, PushRequest{
		RemoteURL: "https://example.com/r.git",
		Branch:    "aip/x; rm -rf /",
	})
	if err == nil || !strings.Contains(err.Error(), "branch") {
		t.Fatalf("err = %v, want branch validation error", err)
	}
}

func TestSandboxPusherRequiresRemote(t *testing.T) {
	if err := (SandboxPusher{}).Push(context.Background(), &fakeSandbox{}, sandbox.Handle{}, PushRequest{Branch: "b"}); err == nil {
		t.Fatal("missing remote accepted")
	}
}

func (f *fakeSandbox) StartProcess(context.Context, sandbox.Handle, []string, map[string]string) (sandbox.Process, error) {
	return nil, fmt.Errorf("not supported in tests")
}
