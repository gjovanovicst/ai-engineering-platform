package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

// ErrBudgetExceeded stops a run before the next model call.
var ErrBudgetExceeded = errors.New("agent: budget exceeded")

// ToolLoopBackend drives an OpenAI-compatible model through a tool-use loop
// inside a sandbox: the model reads, searches, writes and tests until it stops
// calling tools. It is the "real agent" mode for providers that only expose a
// chat API.
type ToolLoopBackend struct {
	Sandbox  sandbox.Sandbox
	Image    string
	BaseURL  string
	APIKey   string
	Model    string
	MaxSteps int
	Client   *http.Client
	// GoBin and Goproxy configure the host-side module fetch (fetch_module).
	GoBin   string
	Goproxy string
	// MCP, when set, adds namespaced MCP tools to the loop.
	MCP *MCPManager
	// MCPConfig is the trusted AIP_MCP_SERVERS JSON for this backend.
	MCPConfig string
}

// Name implements Backend.
func (b *ToolLoopBackend) Name() string { return "openai-tools" }

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Run implements Backend.
func (b *ToolLoopBackend) Run(ctx context.Context, spec Spec, emit func(Event) error) (Result, error) {
	if b.Sandbox == nil {
		return Result{}, fmt.Errorf("agent: tool loop needs a sandbox")
	}
	if b.BaseURL == "" || b.Model == "" {
		return Result{}, fmt.Errorf("agent: tool loop needs a provider base_url and model")
	}
	if spec.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, spec.Timeout)
		defer cancel()
	}
	handle, err := b.Sandbox.Create(ctx, sandbox.Spec{
		Image: b.Image, Workdir: "/workspace", NetworkDisabled: true, Timeout: spec.Timeout,
		Env: map[string]string{
			// PATH: `docker exec` inherits the container's PATH only when the
			// image's shell started with it; an explicit PATH makes `go` and
			// `node` resolvable in every tool call.
			"PATH":       "/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin",
			"HOME":       "/home/agent",
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

	if spec.BudgetWallClock > 0 && (spec.Timeout == 0 || spec.Timeout > spec.BudgetWallClock) {
		// Wall-clock budget is enforced by the context as well.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, spec.BudgetWallClock)
		defer cancel()
	}

	messages := []chatMessage{{Role: "user", Content: spec.Prompt}}
	started := time.Now()
	var usage Result
	for step := 0; step < maxSteps; step++ {
		if err := checkBudget(spec, usage, started); err != nil {
			usage.Provider = "openai-compatible"
			usage.Model = model
			return usage, err
		}
		allowTools := true
		if step == maxSteps-1 {
			// Reserve the last model step for a final answer: without this the
			// agent can spend every step on tools and never emit AIP_PROOF.
			allowTools = false
			messages = append(messages, chatMessage{Role: "user", Content: "This is your final step. Stop calling tools and write your final answer now. If the task asks for a machine-readable final line (AIP_PLAN, AIP_REVIEW or AIP_PROOF), include it exactly once."})
		}
		content, calls, usageStep, err := b.chat(ctx, model, messages, spec.ReadOnly, mcpManager, allowTools)
		if err != nil {
			return Result{}, err
		}
		usage.InputTokens += usageStep.InputTokens
		usage.OutputTokens += usageStep.OutputTokens
		if err := checkBudget(spec, usage, started); err != nil {
			usage.Provider = "openai-compatible"
			usage.Model = model
			return usage, err
		}
		for _, line := range splitLines(content) {
			if err := emit(Event{Type: EventText, Text: line}); err != nil {
				return Result{}, err
			}
		}
		if len(calls) == 0 {
			break
		}
		messages = append(messages, chatMessage{Role: "assistant", Content: content, ToolCalls: calls})
		for _, call := range calls {
			output, terr := b.execute(ctx, tools, call, spec.ReadOnly, mcpManager)
			if terr != nil {
				output = "error: " + terr.Error()
			}
			if call.Function.Name == "fetch_module" {
				_ = emit(Event{Type: "dependency.fetch", Text: truncateText(output, 200)})
			}
			messages = append(messages, chatMessage{Role: "tool", ToolCallID: call.ID, Content: output})
		}
	}

	// Run the repository test command ourselves so the gate has a real exit code
	// (the model may or may not have called run_tests).
	if strings.TrimSpace(spec.TestCommand) != "" {
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		res, terr := b.Sandbox.Exec(testCtx, handle, []string{"sh", "-lc", spec.TestCommand}, nil)
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
	usage.Provider = "openai-compatible"
	usage.Model = model
	if err := emit(Event{Type: EventDone}); err != nil {
		return Result{}, err
	}
	return usage, nil
}

func (b *ToolLoopBackend) execute(ctx context.Context, tools toolbox, call toolCall, readOnly bool, manager *MCPManager) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
		Content string `json:"content"`
		Module  string `json:"module"`
		Version string `json:"version"`
	}
	if raw := strings.TrimSpace(call.Function.Arguments); raw != "" {
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			return "", fmt.Errorf("%s: invalid arguments: %w", call.Function.Name, err)
		}
	}
	switch call.Function.Name {
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
		if manager != nil && strings.HasPrefix(call.Function.Name, "mcp__") {
			return manager.Call(ctx, call.Function.Name, json.RawMessage(call.Function.Arguments))
		}
		return "", fmt.Errorf("unknown tool %q", call.Function.Name)
	}
}

func (b *ToolLoopBackend) diff(ctx context.Context, handle sandbox.Handle) (string, error) {
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

func (b *ToolLoopBackend) chat(ctx context.Context, model string, messages []chatMessage, readOnly bool, manager *MCPManager, allowTools bool) (string, []toolCall, Result, error) {
	request := map[string]any{"model": model, "messages": messages}
	if allowTools {
		request["tools"] = appendMCPTools(toolSpecs(readOnly), manager)
		request["tool_choice"] = "auto"
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return "", nil, Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(b.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", nil, Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", nil, Result{}, fmt.Errorf("agent: provider request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return "", nil, Result{}, fmt.Errorf("agent: provider status %d: %s", res.StatusCode, truncateText(string(body), 500))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []toolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", nil, Result{}, fmt.Errorf("agent: decode provider response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", nil, Result{}, fmt.Errorf("agent: provider returned no choices")
	}
	return out.Choices[0].Message.Content, out.Choices[0].Message.ToolCalls,
		Result{InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens}, nil
}

func checkBudget(spec Spec, usage Result, started time.Time) error {
	if spec.BudgetTokensIn > 0 && usage.InputTokens >= spec.BudgetTokensIn {
		return fmt.Errorf("%w: %d/%d input tokens", ErrBudgetExceeded, usage.InputTokens, spec.BudgetTokensIn)
	}
	if spec.BudgetTokensOut > 0 && usage.OutputTokens >= spec.BudgetTokensOut {
		return fmt.Errorf("%w: %d/%d output tokens", ErrBudgetExceeded, usage.OutputTokens, spec.BudgetTokensOut)
	}
	if spec.BudgetCostMicros > 0 {
		costMicros := int64((float64(usage.InputTokens)*spec.PriceInPer1K + float64(usage.OutputTokens)*spec.PriceOutPer1K) / 1000 * 1e6)
		if costMicros >= spec.BudgetCostMicros {
			return fmt.Errorf("%w: %d/%d micros", ErrBudgetExceeded, costMicros, spec.BudgetCostMicros)
		}
	}
	if spec.BudgetWallClock > 0 && time.Since(started) >= spec.BudgetWallClock {
		return fmt.Errorf("%w: wall clock %s", ErrBudgetExceeded, spec.BudgetWallClock)
	}
	return nil
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// ModelName reports the default model for policy checks.
func (b *ToolLoopBackend) ModelName() string { return b.Model }
