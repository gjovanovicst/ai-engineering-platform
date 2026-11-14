package mcp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestClientHandshakeToolsAndCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.Command(os.Args[0], "-test.run=TestFakeServerProcess")
	cmd.Env = append(os.Environ(), "GO_WANT_MCP_FAKE=1")
	client, err := StartWithEnv(ctx, cmd.Path, cmd.Env, cmd.Args[1:]...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if err := client.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	tools, err := client.ListTools(ctx)
	if err != nil || len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools=%v err=%v", tools, err)
	}
	out, err := client.CallTool(ctx, "echo", json.RawMessage(`{"text":"hi"}`))
	if err != nil || out != "hi" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestFakeServerProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_FAKE") != "1" {
		return
	}
	enc := json.NewEncoder(os.Stdout)
	dec := json.NewDecoder(os.Stdin)
	for {
		var req struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := dec.Decode(&req); err != nil {
			return
		}
		switch req.Method {
		case "initialize":
			_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2024-11-05"}})
		case "tools/list":
			_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"tools": []any{map[string]any{"name": "echo", "description": "echo", "inputSchema": map[string]any{"type": "object"}}}}})
		case "tools/call":
			_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "hi"}}}})
		}
	}
}

func TestClientOverPipe(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()
	go func() {
		enc := json.NewEncoder(serverConn)
		dec := json.NewDecoder(serverConn)
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
				_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"tools": []any{}}})
			}
		}
	}()
	client := NewClient(clientConn, clientConn, func() error { return clientConn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListTools(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}
