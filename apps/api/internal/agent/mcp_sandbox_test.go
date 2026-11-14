package agent

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
)

type fakeMCPSandbox struct {
	proc sandbox.Process
}

func (f *fakeMCPSandbox) Create(context.Context, sandbox.Spec) (sandbox.Handle, error) {
	return sandbox.Handle{ID: "sbx"}, nil
}
func (f *fakeMCPSandbox) Upload(context.Context, sandbox.Handle, []byte) error { return nil }
func (f *fakeMCPSandbox) WriteFile(context.Context, sandbox.Handle, string, []byte) error {
	return nil
}
func (f *fakeMCPSandbox) Exec(context.Context, sandbox.Handle, []string, func(string) error) (sandbox.Result, error) {
	return sandbox.Result{}, nil
}
func (f *fakeMCPSandbox) StartProcess(context.Context, sandbox.Handle, []string, map[string]string) (sandbox.Process, error) {
	return f.proc, nil
}
func (f *fakeMCPSandbox) Export(context.Context, sandbox.Handle) ([]byte, error) { return nil, nil }
func (f *fakeMCPSandbox) Destroy(context.Context, sandbox.Handle) error          { return nil }

type fakeMCPProcess struct {
	in   io.WriteCloser
	out  io.Reader
	done chan struct{}
}

func (p *fakeMCPProcess) Stdin() io.WriteCloser { return p.in }
func (p *fakeMCPProcess) Stdout() io.Reader     { return p.out }
func (p *fakeMCPProcess) Stderr() io.Reader     { return nilReader{} }
func (p *fakeMCPProcess) Wait() error           { <-p.done; return nil }
func (p *fakeMCPProcess) Kill() error {
	select {
	case <-p.done:
	default:
		close(p.done)
	}
	return nil
}

type nilReader struct{}

func (nilReader) Read([]byte) (int, error) { return 0, io.EOF }

// TestMCPInSandboxLifecycle proves the manager is built from a sandbox process
// and its namespaced tool is callable, without requiring Docker.
func TestMCPInSandboxLifecycle(t *testing.T) {
	serverIn, clientIn := io.Pipe()
	clientOut, serverOut := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		enc := json.NewEncoder(serverOut)
		dec := json.NewDecoder(serverIn)
		for {
			var req struct {
				ID     int    `json:"id"`
				Method string `json:"method"`
			}
			if err := dec.Decode(&req); err != nil {
				return
			}
			switch req.Method {
			case "initialize":
				_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{}})
			case "tools/list":
				_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
					"tools": []any{map[string]any{"name": "echo", "description": "echo", "inputSchema": map[string]any{"type": "object"}}},
				}})
			case "tools/call":
				_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
					"content": []any{map[string]any{"type": "text", "text": "from-sandbox"}},
				}})
			}
		}
	}()
	proc := &fakeMCPProcess{in: clientIn, out: clientOut, done: done}
	manager, failures := StartMCPInSandbox(context.Background(), &fakeMCPSandbox{proc: proc}, sandbox.Handle{},
		[]MCPServer{{Name: "fs", Command: "mcp-fs"}})
	if len(failures) != 0 {
		t.Fatalf("failures = %v", failures)
	}
	defer manager.Close()
	if len(manager.Specs()) != 1 {
		t.Fatalf("specs = %v", manager.Specs())
	}
	out, err := manager.Call(context.Background(), "mcp__fs__echo", json.RawMessage(`{}`))
	if err != nil || out != "from-sandbox" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
