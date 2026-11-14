package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicToolLoop(t *testing.T) {
	var calls int
	var lastBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "secret" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing anthropic headers")
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &lastBody)
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = w.Write([]byte(`{"content":[{"type":"tool_use","id":"tu_1","name":"read_file","input":{"path":"main.go"}}],"usage":{"input_tokens":10,"output_tokens":5}}`))
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"done"}],"usage":{"input_tokens":7,"output_tokens":3}}`))
	}))
	defer server.Close()

	sb := &toolSandbox{}
	backend := &AnthropicBackend{Sandbox: sb, BaseURL: server.URL, APIKey: "secret", Model: "claude-test"}
	var texts []string
	result, err := backend.Run(context.Background(), Spec{Prompt: "fix it"}, func(ev Event) error {
		if ev.Type == EventText {
			texts = append(texts, ev.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
	if strings.Join(texts, "") != "done" {
		t.Fatalf("texts = %v", texts)
	}
	if result.InputTokens != 17 || result.OutputTokens != 8 {
		t.Fatalf("usage = %+v", result)
	}
	read := false
	for _, exec := range sb.execs {
		if strings.Contains(strings.Join(exec, " "), "cat") {
			read = true
		}
	}
	if !read {
		t.Fatalf("tool was not executed: %v", sb.execs)
	}
	tools, _ := lastBody["tools"].([]any)
	if len(tools) == 0 {
		t.Fatal("tools missing from the request")
	}
	if _, ok := tools[0].(map[string]any)["input_schema"]; !ok {
		t.Fatalf("tool schema shape = %v", tools[0])
	}
}
