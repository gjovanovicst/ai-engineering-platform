package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

// PushRequest describes a branch to publish from a sandbox.
type PushRequest struct {
	RemoteURL string // may embed a token; never logged
	Branch    string
	Message   string
}

// Pusher commits the sandbox workspace and pushes it to the remote.
type Pusher interface {
	Push(ctx context.Context, sb sandbox.Sandbox, h sandbox.Handle, req PushRequest) error
}

// SandboxPusher runs git inside the run's sandbox (the only place the
// workspace exists). It never logs the remote URL because it may contain a token.
type SandboxPusher struct{}

// Push implements Pusher.
func (SandboxPusher) Push(ctx context.Context, sb sandbox.Sandbox, h sandbox.Handle, req PushRequest) error {
	if req.RemoteURL == "" || req.Branch == "" {
		return fmt.Errorf("git: push needs a remote url and a branch")
	}
	// The URL and branch are validated below; the script is fixed except for
	// the quoted arguments, so no shell interpolation of model output happens.
	if strings.ContainsAny(req.Branch, " \t\n;|&$`") {
		return fmt.Errorf("git: invalid branch name")
	}
	script := fmt.Sprintf(
		`git add -A && git -c user.email=aip@local -c user.name=aip commit -q -m %q; git checkout -q -b %q && git push -q %q HEAD:refs/heads/%s`,
		req.Message, req.Branch, req.RemoteURL, req.Branch,
	)
	res, err := sb.Exec(ctx, h, []string{"sh", "-lc", script}, nil)
	if err != nil {
		return fmt.Errorf("git: push: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git: push exited with %d", res.ExitCode)
	}
	return nil
}
