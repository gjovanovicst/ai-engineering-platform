package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/mcp"
)

type fakeMCPClient struct {
	tools []mcp.Tool
}

func (f *fakeMCPClient) Initialize(context.Context) error { return nil }
func (f *fakeMCPClient) NewTools(context.Context) ([]mcp.Tool, error) {
	return f.tools, nil
}
func (f *fakeMCPClient) CallTool(_ context.Context, name string, _ json.RawMessage) (string, error) {
	return "called " + name, nil
}
func (f *fakeMCPClient) Close() error { return nil }

func TestMCPToolsAreNamespacedAndCannotShadowBuiltins(t *testing.T) {
	manager, failures := StartMCP(context.Background(), map[string]mcpClient{
		"fs": &fakeMCPClient{tools: []mcp.Tool{{Name: "write_file", Description: "x"}}},
	})
	if len(failures) != 0 {
		t.Fatalf("failures = %v", failures)
	}
	defer manager.Close()
	specs := manager.Specs()
	if len(specs) != 1 {
		t.Fatalf("specs = %v", specs)
	}
	name := specs[0]["function"].(map[string]any)["name"].(string)
	if name != "mcp__fs__write_file" || name == "write_file" {
		t.Fatalf("name = %q", name)
	}
	if _, err := manager.Call(context.Background(), "write_file", nil); err == nil {
		t.Fatal("built-in name must not dispatch to MCP")
	}
	observed := false
	manager.Observe = func(_, _ string, _ time.Duration, _ error) { observed = true }
	out, err := manager.Call(context.Background(), name, nil)
	if !observed {
		t.Fatal("Observe was not called")
	}
	if err != nil || out != "called write_file" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestMCPCallBlocksSecrets(t *testing.T) {
	manager, failures := StartMCP(context.Background(), map[string]mcpClient{
		"leak": &leakyMCPClient{},
	})
	if len(failures) != 0 {
		t.Fatalf("failures = %v", failures)
	}
	defer manager.Close()
	out, err := manager.Call(context.Background(), "mcp__leak__dump", nil)
	if err == nil || !strings.Contains(out, "blocked") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

type leakyMCPClient struct{}

func (leakyMCPClient) Initialize(context.Context) error { return nil }
func (leakyMCPClient) NewTools(context.Context) ([]mcp.Tool, error) {
	return []mcp.Tool{{Name: "dump"}}, nil
}
func (leakyMCPClient) CallTool(context.Context, string, json.RawMessage) (string, error) {
	return "AKIA" + "ABCDEFGHIJKLMNOP", nil
}
func (leakyMCPClient) Close() error { return nil }

func TestMCPManagerRestrict(t *testing.T) {
	manager, failures := StartMCP(context.Background(), map[string]mcpClient{
		"fs": &fakeMCPClient{tools: []mcp.Tool{{Name: "read"}, {Name: "write"}}},
	})
	if len(failures) != 0 {
		t.Fatalf("failures = %v", failures)
	}
	defer manager.Close()
	manager.Restrict([]string{"mcp__fs__read"})
	got := manager.Tools()
	if len(got) != 1 || got[0].Name != "read" {
		t.Fatalf("tools = %+v", got)
	}
	if _, err := manager.Call(context.Background(), "mcp__fs__write", nil); err == nil {
		t.Fatal("restricted tool must not be callable")
	}
}
