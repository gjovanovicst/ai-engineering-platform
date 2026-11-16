package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

// toolbox runs the tools an agent may use against a live sandbox.
type toolbox interface {
	Read(ctx context.Context, path string) (string, error)
	List(ctx context.Context, path string) (string, error)
	Grep(ctx context.Context, pattern string) (string, error)
	Write(ctx context.Context, path, content string) error
	Test(ctx context.Context) (string, error)
	// Fetch downloads a Go module on the host and installs it in the sandbox's
	// offline module cache.
	Fetch(ctx context.Context, module, version string) (string, error)
}

type sandboxToolbox struct {
	sandbox     sandbox.Sandbox
	handle      sandbox.Handle
	testCommand string
	goBin       string
	goproxy     string
}

func (t *sandboxToolbox) run(ctx context.Context, script string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	argv := append([]string{"sh", "-lc", script, "sh"}, args...)
	res, err := t.sandbox.Exec(ctx, t.handle, argv, nil)
	if err != nil {
		return "", err
	}
	out := res.Stdout
	if strings.TrimSpace(res.Stderr) != "" {
		out += "\nstderr:\n" + res.Stderr
	}
	if len(out) > maxToolOutput {
		out = out[:maxToolOutput] + "\n… (truncated)"
	}
	return out, nil
}

func (t *sandboxToolbox) Read(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("read_file: path is required")
	}
	return t.run(ctx, `cat -- "$1"`, path)
}

func (t *sandboxToolbox) List(ctx context.Context, path string) (string, error) {
	if path == "" {
		path = "."
	}
	return t.run(ctx, `ls -la -- "$1"`, path)
}

func (t *sandboxToolbox) Grep(ctx context.Context, pattern string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("grep: pattern is required")
	}
	return t.run(ctx, `grep -rn -- "$1" . | head -200`, pattern)
}

func (t *sandboxToolbox) Write(ctx context.Context, path, content string) error {
	if path == "" {
		return fmt.Errorf("write_file: path is required")
	}
	return t.sandbox.WriteFile(ctx, t.handle, path, []byte(content))
}

func (t *sandboxToolbox) Fetch(ctx context.Context, module, version string) (string, error) {
	return fetchModule(ctx, t.sandbox, t.handle, t.goBin, t.goproxy, module, version)
}

func (t *sandboxToolbox) Test(ctx context.Context) (string, error) {
	command := strings.TrimSpace(t.testCommand)
	if command == "" {
		return "", fmt.Errorf("run_tests: no test command is configured")
	}
	return t.run(ctx, command)
}

const (
	maxToolOutput = 20000
	maxDiffBytes  = 1 << 20
)

// toolSpecs is the OpenAI function-calling schema for the tools above.
func toolSpecs(readOnly bool) []map[string]any {
	obj := func(props map[string]any, required ...string) map[string]any {
		if required == nil {
			required = []string{} // JSON schema requires an array, not null
		}
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	fn := func(name, description string, parameters map[string]any) map[string]any {
		return map[string]any{"type": "function", "function": map[string]any{
			"name": name, "description": description, "parameters": parameters,
		}}
	}
	specs := []map[string]any{
		fn("read_file", "Read a file from the repository.", obj(map[string]any{"path": map[string]any{"type": "string"}}, "path")),
		fn("list_files", "List files in a directory (default the repo root).", obj(map[string]any{"path": map[string]any{"type": "string"}})),
		fn("grep", "Search the repository with grep -rn.", obj(map[string]any{"pattern": map[string]any{"type": "string"}}, "pattern")),
		fn("run_tests", "Run the project's test command and return the output.", obj(map[string]any{})),
	}
	if !readOnly {
		specs = append(specs, fn("write_file", "Create or overwrite a file with the given content.", obj(map[string]any{
			"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"},
		}, "path", "content")))
		specs = append(specs, fn("fetch_module",
			"Download a Go module into the sandbox's offline module cache. The worker host fetches it; the sandbox has no network. Use this when go build or go test reports a missing module instead of vendoring or reimplementing it. Example: module golang.org/x/crypto, version v0.36.0.",
			obj(map[string]any{
				"module":  map[string]any{"type": "string", "description": "Go module path, e.g. golang.org/x/crypto"},
				"version": map[string]any{"type": "string", "description": "module version, e.g. v0.36.0"},
			}, "module", "version")))
	}
	return specs
}
