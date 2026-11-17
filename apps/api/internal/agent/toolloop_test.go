package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

type toolSandbox struct {
	mu      sync.Mutex
	execs   [][]string
	written map[string]string
	diff    string
}

func (f *toolSandbox) Create(context.Context, sandbox.Spec) (sandbox.Handle, error) {
	return sandbox.Handle{ID: "h"}, nil
}
func (f *toolSandbox) Upload(context.Context, sandbox.Handle, []byte) error { return nil }
func (f *toolSandbox) WriteFile(_ context.Context, _ sandbox.Handle, path string, content []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.written == nil {
		f.written = map[string]string{}
	}
	f.written[path] = string(content)
	return nil
}
func (f *toolSandbox) Exec(_ context.Context, _ sandbox.Handle, cmd []string, onLine func(string) error) (sandbox.Result, error) {
	f.mu.Lock()
	f.execs = append(f.execs, cmd)
	diff := f.diff
	f.mu.Unlock()
	if strings.Contains(strings.Join(cmd, " "), "diff --cached") {
		return sandbox.Result{Stdout: diff}, nil
	}
	if len(cmd) > 4 && cmd[0] == "sh" && strings.Contains(cmd[2], "cat") {
		return sandbox.Result{Stdout: "package main\n"}, nil
	}
	if len(cmd) >= 3 && cmd[0] == "sh" && strings.Contains(cmd[2], "go test") {
		return sandbox.Result{ExitCode: 1, Stdout: "FAIL example.com/customer"}, nil
	}
	if onLine != nil {
		_ = onLine("ok")
	}
	return sandbox.Result{}, nil
}
func (f *toolSandbox) Export(context.Context, sandbox.Handle) ([]byte, error) {
	return []byte("workspace-tar"), nil
}
func (f *toolSandbox) Destroy(context.Context, sandbox.Handle) error { return nil }

func TestToolLoopReadsThenAnswers(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Messages []chatMessage `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if calls == 1 {
			_, _ = w.Write([]byte(`{
				"choices":[{"message":{"content":"","tool_calls":[{"id":"call-1","type":"function",
					"function":{"name":"read_file","arguments":"{\"path\":\"README.md\"}"}}]}}],
				"usage":{"prompt_tokens":10,"completion_tokens":2}
			}`))
			return
		}
		if len(req.Messages) == 0 || req.Messages[len(req.Messages)-1].Role != "tool" ||
			req.Messages[len(req.Messages)-1].Content == "" {
			t.Errorf("tool result not sent back to the model: %+v", req.Messages)
		}
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"done: read the file"}}],
			"usage":{"prompt_tokens":5,"completion_tokens":3}
		}`))
	}))
	defer server.Close()

	fs := &toolSandbox{diff: "diff --git a/x b/x"}
	backend := &ToolLoopBackend{Sandbox: fs, BaseURL: server.URL, Model: "m", MaxSteps: 5}
	var text []string
	result, err := backend.Run(context.Background(), Spec{
		Prompt: "read README", CaptureWorkspace: true, Timeout: 0,
	}, func(e Event) error {
		if e.Type == EventText {
			text = append(text, e.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(text) != 1 || text[0] != "done: read the file" {
		t.Fatalf("text = %v", text)
	}
	if result.Diff != fs.diff || string(result.Workspace) != "workspace-tar" {
		t.Fatalf("result diff=%q workspace=%q", result.Diff, result.Workspace)
	}
	if result.InputTokens != 15 || result.OutputTokens != 5 {
		t.Fatalf("usage = %d/%d, want 15/5", result.InputTokens, result.OutputTokens)
	}
	read := false
	for _, cmd := range fs.execs {
		if len(cmd) > 0 && cmd[len(cmd)-1] == "README.md" {
			read = true
		}
	}
	if !read {
		t.Fatalf("read_file not executed: %v", fs.execs)
	}
}

func TestToolLoopStopsOnBudget(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"spent too much"}}],
			"usage":{"prompt_tokens":10,"completion_tokens":0}
		}`))
	}))
	defer server.Close()

	fs := &toolSandbox{}
	backend := &ToolLoopBackend{Sandbox: fs, BaseURL: server.URL, Model: "m", MaxSteps: 5}
	result, err := backend.Run(context.Background(), Spec{Prompt: "go", BudgetTokensIn: 5},
		func(Event) error { return nil })
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("err = %v, want budget exceeded", err)
	}
	if calls != 1 {
		t.Fatalf("provider calls = %d, want 1", calls)
	}
	if result.InputTokens != 10 {
		t.Fatalf("partial usage = %+v", result)
	}
}

func TestToolSpecsUseJSONSchemaArrays(t *testing.T) {
	for _, spec := range toolSpecs(false) {
		params := spec["function"].(map[string]any)["parameters"].(map[string]any)
		if params["required"] == nil {
			t.Fatalf("tool %v has required:null", spec["function"].(map[string]any)["name"])
		}
		if _, ok := params["required"].([]string); !ok {
			t.Fatalf("tool %v required is %T", spec["function"].(map[string]any)["name"], params["required"])
		}
	}
}

func TestAssistantToolCallMessagesCarryContent(t *testing.T) {
	raw, err := json.Marshal(chatMessage{Role: "assistant", ToolCalls: []toolCall{{ID: "1", Type: "function"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"content":""`) {
		t.Fatalf("assistant message must include content: %s", raw)
	}
}

func TestToolLoopRunsTheTestCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"done"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer server.Close()

	fs := &toolSandbox{}
	backend := &ToolLoopBackend{Sandbox: fs, BaseURL: server.URL, Model: "m", MaxSteps: 2}
	result, err := backend.Run(context.Background(), Spec{Prompt: "x", TestCommand: "go test ./..."},
		func(Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if result.TestExitCode != 1 || !strings.Contains(result.TestOutput, "FAIL") {
		t.Fatalf("test result = %+v", result)
	}
}

func (s *toolSandbox) StartProcess(context.Context, sandbox.Handle, []string, map[string]string) (sandbox.Process, error) {
	return nil, fmt.Errorf("not supported in tests")
}

func TestFinalStepChatOmitsTools(t *testing.T) {
	t.Parallel()
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"AIP_PROOF: {\"evidence\":[]}"}}]}`))
	}))
	defer srv.Close()
	b := &ToolLoopBackend{BaseURL: srv.URL, Model: "m"}
	content, calls, _, err := b.chat(context.Background(), "m", []chatMessage{{Role: "user", Content: "finish"}}, false, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["tools"]; ok {
		t.Fatal("the final step must not offer tools")
	}
	if _, ok := body["tool_choice"]; ok {
		t.Fatal("the final step must not force a tool choice")
	}
	if len(calls) != 0 || !strings.Contains(content, "AIP_PROOF") {
		t.Fatalf("content = %q, calls = %v", content, calls)
	}
}
