package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

type fakeSandbox struct {
	created   bool
	destroyed bool
	spec      sandbox.Spec
	files     map[string][]byte
	commands  [][]string
	lines     []string
	exitCode  int
	err       error
}

func (f *fakeSandbox) Create(_ context.Context, spec sandbox.Spec) (sandbox.Handle, error) {
	f.created = true
	f.spec = spec
	return sandbox.Handle{ID: "sbx-1"}, nil
}

func (f *fakeSandbox) Upload(context.Context, sandbox.Handle, []byte) error { return nil }

func (f *fakeSandbox) WriteFile(_ context.Context, _ sandbox.Handle, path string, content []byte) error {
	if f.files == nil {
		f.files = map[string][]byte{}
	}
	f.files[path] = content
	return f.err
}

func (f *fakeSandbox) Exec(_ context.Context, _ sandbox.Handle, cmd []string, onLine func(string) error) (sandbox.Result, error) {
	f.commands = append(f.commands, cmd)
	if f.err != nil {
		return sandbox.Result{}, f.err
	}
	if onLine != nil {
		for _, line := range f.lines {
			if err := onLine(line); err != nil {
				return sandbox.Result{}, err
			}
		}
	}
	return sandbox.Result{ExitCode: f.exitCode}, nil
}

func (f *fakeSandbox) Export(context.Context, sandbox.Handle) ([]byte, error) {
	return []byte("tar"), nil
}

func (f *fakeSandbox) Destroy(context.Context, sandbox.Handle) error {
	f.destroyed = true
	return nil
}

func TestCommandBackendRun(t *testing.T) {
	sb := &fakeSandbox{lines: []string{"step 1", "step 2"}}
	backend, err := NewCommandBackend(sb, "agent --print", "agent:latest")
	if err != nil {
		t.Fatal(err)
	}

	var events []Event
	_, err = backend.Run(context.Background(), Spec{Prompt: "implement SSO", RunID: "run-1"},
		func(e Event) error {
			events = append(events, e)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}

	if !sb.created || !sb.destroyed {
		t.Fatalf("sandbox lifecycle: created=%v destroyed=%v", sb.created, sb.destroyed)
	}
	if !sb.spec.NetworkDisabled {
		t.Fatal("sandbox must run without network by default")
	}
	if got := string(sb.files[promptPath]); got != "implement SSO" {
		t.Fatalf("prompt file = %q", got)
	}
	found := false
	for _, cmd := range sb.commands {
		if len(cmd) == 3 && cmd[2] == "agent --print" {
			found = true
		}
	}
	if !found {
		t.Fatalf("agent command not executed: %v", sb.commands)
	}
	if len(events) != 3 || events[2].Type != EventDone {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Text != "step 1" {
		t.Fatalf("first event = %+v", events[0])
	}
}

func TestCommandBackendNonZeroExit(t *testing.T) {
	sb := &fakeSandbox{exitCode: 2}
	backend, _ := NewCommandBackend(sb, "agent", "agent:latest")

	_, err := backend.Run(context.Background(), Spec{Prompt: "x"}, func(Event) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "exited with 2") {
		t.Fatalf("err = %v", err)
	}
	if !sb.destroyed {
		t.Fatal("sandbox must be destroyed on failure")
	}
}

func TestCommandBackendRejectsBadConfig(t *testing.T) {
	if _, err := NewCommandBackend(nil, "agent", "img"); err == nil {
		t.Fatal("nil sandbox accepted")
	}
	if _, err := NewCommandBackend(&fakeSandbox{}, "", "img"); err == nil {
		t.Fatal("empty command accepted")
	}
	if _, err := NewCommandBackend(&fakeSandbox{}, "agent", ""); err == nil {
		t.Fatal("empty image accepted")
	}
}

func TestCommandBackendPropagatesEmitError(t *testing.T) {
	sb := &fakeSandbox{lines: []string{"one", "two"}}
	backend, _ := NewCommandBackend(sb, "agent", "img")

	sentinel := errors.New("stop")
	_, err := backend.Run(context.Background(), Spec{Prompt: "x"}, func(Event) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the emit error", err)
	}
	if !sb.destroyed {
		t.Fatal("sandbox must be destroyed when the consumer stops")
	}
}

func (s *fakeSandbox) StartProcess(context.Context, sandbox.Handle, []string, map[string]string) (sandbox.Process, error) {
	return nil, fmt.Errorf("not supported in tests")
}
