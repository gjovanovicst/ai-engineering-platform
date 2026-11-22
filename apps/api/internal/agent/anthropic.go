package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

// AnthropicBackend drives an Anthropic Messages API model through the same
// sandbox tool loop as ToolLoopBackend.
type AnthropicBackend struct {
	Sandbox  sandbox.Sandbox
	Image    string
	BaseURL  string
	APIKey   string
	Model    string
	MaxSteps int
	Client   *http.Client
	// MCP adds namespaced MCP tools to the Messages loop.
	MCP *MCPManager
	// MCPConfig is the trusted AIP_MCP_SERVERS JSON for this backend.
	MCPConfig string // GoBin and Goproxy configure the host-side module fetch (fetch_module).
	GoBin     string
	Goproxy   string
}

// Name implements Backend.
func (b *AnthropicBackend) Name() string { return "anthropic-messages" }

type anthBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthMessage struct {
	Role    string      `json:"role"`
	Content []anthBlock `json:"content"`
}

// Run implements Backend.
func (b *AnthropicBackend) Run(ctx context.Context, spec Spec, emit func(Event) error) (Result, error) {
	if b.Sandbox == nil {
		return Result{}, fmt.Errorf("agent: anthropic backend needs a sandbox")
	}
	if b.BaseURL == "" || b.Model == "" {
		return Result{}, fmt.Errorf("agent: anthropic backend needs base_url and model")
	}
	if spec.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, spec.Timeout)
		defer cancel()
	}
	handle, err := b.Sandbox.Create(ctx, sandbox.Spec{
		Image: b.Image, Workdir: "/workspace", NetworkDisabled: true, Timeout: spec.Timeout,
		Env: map[string]string{
			"GOMODCACHE": moduleCacheDir,
			"GOPROXY":    "off",
			"GOSUMDB":    "off",
			"GOFLAGS":    "-mod=mod",
		},
	})
	if err != nil {
		return Result{}, fmt.Errorf("agent: create sandbox: %w", err)
	}
	defer func() { _ = b.Sandbox.Destroy(context.WithoutCancel(ctx), handle) }()

	if len(spec.Workspace) > 0 {
		if err := b.Sandbox.Upload(ctx, handle, spec.Workspace); err != nil {
			return Result{}, fmt.Errorf("agent: upload workspace: %w", err)
		}
	} else {
		_, _ = b.Sandbox.Exec(ctx, handle, []string{"sh", "-lc",
			"git init -q && git config user.email aip@local && git config user.name aip && git add -A && git commit -q --allow-empty -m baseline"}, nil)
	}
	_, _ = b.Sandbox.Exec(ctx, handle, []string{"sh", "-lc",
		"git config user.email aip@local && git config user.name aip"}, nil)

	tools := toolbox(&sandboxToolbox{
		sandbox: b.Sandbox, handle: handle, testCommand: spec.TestCommand,
		goBin: b.GoBin, goproxy: b.Goproxy,
	})
	mcpManager, mcpFailures := StartMCPIsolated(ctx, b.Sandbox, b.Image, mustMCPServers(effectiveMCPConfig(spec.MCPConfig, b.MCPConfig)))
	if mcpManager != nil {
		defer mcpManager.Close()
	}
	for name, err := range mcpFailures {
		_ = emit(Event{Type: "mcp.unavailable", Text: name + ": " + err.Error()})
	}
	if mcpManager != nil {
		mcpManager.Restrict(spec.AllowedTools)
		if tools := mcpManager.Tools(); len(tools) > 0 {
			if raw, merr := json.Marshal(tools); merr == nil {
				_ = emit(Event{Type: "mcp.tools", Text: string(raw)})
			}
		}
		mcpManager.Observe = func(server, tool string, duration time.Duration, err error) {
			status := "ok"
			if err != nil {
				status = "error"
			}
			_ = emit(Event{Type: "mcp.call", Text: fmt.Sprintf("%s/%s %s %dms", server, tool, status, duration.Milliseconds())})
		}
	}
	model := b.Model
	if spec.Model != "" {
		model = spec.Model
	}
	maxSteps := b.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 12
	}

	messages := []anthMessage{{Role: "user", Content: []anthBlock{{Type: "text", Text: spec.Prompt}}}}
	started := time.Now()
	var usage Result
	for step := 0; step < maxSteps; step++ {
		if err := checkBudget(spec, usage, started); err != nil {
			usage.Provider = "anthropic"
			usage.Model = model
			return usage, err
		}
		blocks, stepUsage, err := b.message(ctx, model, messages, tools, spec.ReadOnly, mcpManager)
		if err != nil {
			return Result{}, err
		}
		usage.InputTokens += stepUsage.InputTokens
		usage.OutputTokens += stepUsage.OutputTokens
		if err := checkBudget(spec, usage, started); err != nil {
			usage.Provider = "anthropic"
			usage.Model = model
			return usage, err
		}

		var calls []anthBlock
		for _, block := range blocks {
			if block.Type == "text" {
				for _, line := range splitLines(block.Text) {
					if err := emit(Event{Type: EventText, Text: line}); err != nil {
						return Result{}, err
					}
				}
			}
			if block.Type == "tool_use" {
				calls = append(calls, block)
			}
		}
		if len(calls) == 0 {
			break
		}
		messages = append(messages, anthMessage{Role: "assistant", Content: blocks})
		results := make([]anthBlock, 0, len(calls))
		for _, call := range calls {
			output, terr := b.execute(ctx, tools, call, spec.ReadOnly, mcpManager)
			if terr != nil {
				output = "error: " + terr.Error()
			}
			if call.Name == "fetch_module" {
				_ = emit(Event{Type: "dependency.fetch", Text: truncateText(output, 200)})
			}
			results = append(results, anthBlock{Type: "tool_result", ToolUseID: call.ID, Content: output})
		}
		messages = append(messages, anthMessage{Role: "user", Content: results})
	}

	if strings.TrimSpace(spec.TestCommand) != "" {
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		res, terr := b.Sandbox.Exec(testCtx, handle, toolchainShell(spec.TestCommand), nil)
		cancel()
		if terr == nil {
			usage.TestExitCode = res.ExitCode
			usage.TestOutput = truncateText(res.Stdout+"\n"+res.Stderr, 20000)
			_ = emit(Event{Type: "test.completed", Text: fmt.Sprintf("exit %d", res.ExitCode)})
		}
	}

	diff, err := b.diff(ctx, handle)
	if err != nil {
		return Result{}, err
	}
	var workspace []byte
	if spec.CaptureWorkspace {
		exportCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		workspace, err = b.Sandbox.Export(exportCtx, handle)
		if err != nil {
			return Result{}, fmt.Errorf("agent: export workspace: %w", err)
		}
	}
	usage.Diff = diff
	usage.Workspace = workspace
	usage.Provider = "anthropic"
	usage.Model = model
	if err := emit(Event{Type: EventDone}); err != nil {
		return Result{}, err
	}
	return usage, nil
}

func (b *AnthropicBackend) execute(ctx context.Context, tools toolbox, call anthBlock, readOnly bool, manager *MCPManager) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
		Content string `json:"content"`
		Module  string `json:"module"`
		Version string `json:"version"`
	}
	if len(call.Input) > 0 {
		if err := json.Unmarshal(call.Input, &args); err != nil {
			return "", fmt.Errorf("%s: invalid input: %w", call.Name, err)
		}
	}
	switch call.Name {
	case "read_file":
		return tools.Read(ctx, args.Path)
	case "list_files":
		return tools.List(ctx, args.Path)
	case "grep":
		return tools.Grep(ctx, args.Pattern)
	case "write_file":
		if readOnly {
			return "", fmt.Errorf("write_file is not available to this agent")
		}
		return "written", tools.Write(ctx, args.Path, args.Content)
	case "run_tests":
		return tools.Test(ctx)
	case "fetch_module":
		if readOnly {
			return "", fmt.Errorf("fetch_module is not available to this agent")
		}
		return tools.Fetch(ctx, args.Module, args.Version)
	default:
		if manager != nil && strings.HasPrefix(call.Name, "mcp__") {
			return manager.Call(ctx, call.Name, call.Input)
		}
		return "", fmt.Errorf("unknown tool %q", call.Name)
	}
}

func (b *AnthropicBackend) diff(ctx context.Context, handle sandbox.Handle) (string, error) {
	if _, err := b.Sandbox.Exec(ctx, handle, []string{"git", "add", "-A"}, nil); err != nil {
		return "", fmt.Errorf("agent: stage changes: %w", err)
	}
	res, err := b.Sandbox.Exec(ctx, handle, []string{"git", "--no-pager", "diff", "--cached"}, nil)
	if err != nil {
		return "", fmt.Errorf("agent: diff: %w", err)
	}
	if len(res.Stdout) > maxDiffBytes {
		return "", fmt.Errorf("agent: diff exceeds %d bytes", maxDiffBytes)
	}
	return res.Stdout, nil
}

func (b *AnthropicBackend) message(ctx context.Context, model string, messages []anthMessage, tools toolbox, readOnly bool, manager *MCPManager) ([]anthBlock, Result, error) {
	payload := map[string]any{
		"model": model, "max_tokens": 8192, "messages": messages,
		"tools": anthropicTools(readOnly, manager),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(b.BaseURL, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", b.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, Result{}, fmt.Errorf("agent: anthropic request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return nil, Result{}, fmt.Errorf("agent: anthropic status %d: %s", res.StatusCode, truncateText(string(raw), 500))
	}
	var out struct {
		Content []anthBlock `json:"content"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, Result{}, fmt.Errorf("agent: decode anthropic response: %w", err)
	}
	return out.Content, Result{InputTokens: out.Usage.InputTokens, OutputTokens: out.Usage.OutputTokens}, nil
}

// anthropicTools converts the shared tool specs to the Messages API shape.
func anthropicTools(readOnly bool, manager *MCPManager) []map[string]any {
	out := make([]map[string]any, 0)
	for _, spec := range appendMCPTools(toolSpecs(readOnly), manager) {
		fn := spec["function"].(map[string]any)
		out = append(out, map[string]any{
			"name": fn["name"], "description": fn["description"], "input_schema": fn["parameters"],
		})
	}
	return out
}

// ModelName reports the default model for policy checks.
func (b *AnthropicBackend) ModelName() string { return b.Model }
