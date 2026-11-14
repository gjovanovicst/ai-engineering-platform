package agent

import (
	"context"

	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

const promptPath = "/workspace/.aip/prompt.txt"

// CommandBackend runs an external agent CLI (Claude Code, Codex, ...) inside a
// sandbox and streams its stdout. The command is trusted configuration, never
// model output.
type CommandBackend struct {
	sandbox sandbox.Sandbox
	command string
	image   string
}

// NewCommandBackend validates and returns the backend.
func NewCommandBackend(sb sandbox.Sandbox, command, image string) (*CommandBackend, error) {
	if sb == nil {
		return nil, errors.New("agent: sandbox is required")
	}
	if command == "" {
		return nil, errors.New("agent: command is required")
	}
	if image == "" {
		return nil, errors.New("agent: image is required")
	}
	return &CommandBackend{sandbox: sb, command: command, image: image}, nil
}

// Name implements Backend.
func (b *CommandBackend) Name() string { return "command" }

// Run creates a sandbox, writes the prompt, executes the agent and destroys the
// sandbox. Cancellation tears the sandbox down through a separate context.
func (b *CommandBackend) Run(ctx context.Context, spec Spec, emit func(Event) error) (Result, error) {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	image := spec.Image
	if image == "" {
		image = b.image
	}

	handle, err := b.sandbox.Create(runCtx, sandbox.Spec{
		Image:           image,
		Workdir:         "/workspace",
		NetworkDisabled: true,
		CPU:             2,
		MemoryMB:        4096,
		Timeout:         timeout,
	})
	if err != nil {
		return Result{}, fmt.Errorf("agent: create sandbox: %w", err)
	}
	defer func() {
		// Deliberately not the run context: cleanup must survive cancellation.
		_ = b.sandbox.Destroy(context.WithoutCancel(ctx), handle)
	}()

	if err := b.sandbox.WriteFile(runCtx, handle, promptPath, []byte(spec.Prompt)); err != nil {
		return Result{}, fmt.Errorf("agent: write prompt: %w", err)
	}

	if len(spec.Workspace) > 0 {
		if err := b.sandbox.Upload(runCtx, handle, spec.Workspace); err != nil {
			return Result{}, fmt.Errorf("agent: upload workspace: %w", err)
		}
	} else {
		// Baseline so that the produced diff is relative to an empty workspace.
		_, _ = b.sandbox.Exec(runCtx, handle, []string{"sh", "-lc",
			"git init -q && git config user.email aip@local && git config user.name aip && git add -A && git commit -q --allow-empty -m baseline"}, nil)
	}

	agentCtx, span := otel.Tracer("aip/agent").Start(runCtx, "agent.execute",
		trace.WithAttributes(attribute.String("run.id", spec.RunID)))
	execResult, err := b.sandbox.Exec(agentCtx, handle, []string{"sh", "-lc", b.command}, func(line string) error {
		return emit(Event{Type: EventText, Text: line})
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "exec failed")
		span.End()
		return Result{}, err
	}
	if execResult.ExitCode != 0 {
		span.SetStatus(codes.Error, "agent failed")
		span.End()
		return Result{}, fmt.Errorf("agent: command exited with %d: %s", execResult.ExitCode, execResult.Stderr)
	}
	span.SetAttributes(attribute.Int("agent.exit_code", execResult.ExitCode))
	span.End()

	diff, err := b.sandbox.Exec(runCtx, handle, []string{"sh", "-lc", "git add -A && git diff --cached"}, nil)
	if err != nil {
		return Result{}, err
	}

	out := Result{Diff: diff.Stdout}
	if spec.CaptureWorkspace {
		workspace, werr := b.sandbox.Export(runCtx, handle)
		if werr != nil {
			return Result{}, werr
		}
		out.Workspace = workspace
	}
	if spec.TestCommand != "" {
		tests, err := b.sandbox.Exec(runCtx, handle, []string{"sh", "-lc", spec.TestCommand}, nil)
		if err != nil {
			return Result{}, err
		}
		out.TestExitCode = tests.ExitCode
		out.TestOutput = tests.Stdout + tests.Stderr
		_ = emit(Event{Type: "test.completed", Text: tests.Stdout})
	}
	if err := emit(Event{Type: EventDone}); err != nil {
		return Result{}, err
	}
	return out, nil
}

// ModelName reports the command backend as "command" for model policy.
func (b *CommandBackend) ModelName() string { return "command" }
