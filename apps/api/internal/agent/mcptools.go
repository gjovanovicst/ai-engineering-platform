package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/secretscan"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/mcp"
)

// mcpClient is the subset of mcp.Client the manager needs (fakeable in tests).
type mcpClient interface {
	Initialize(ctx context.Context) error
	NewTools(ctx context.Context) ([]mcp.Tool, error)
	CallTool(ctx context.Context, name string, args json.RawMessage) (string, error)
	Close() error
}

// NewTools avoids a name clash with mcp.Client.ListTools in adapter types; the
// concrete *mcp.Client is wrapped by clientAdapter below.
type clientAdapter struct{ *mcp.Client }

func (c clientAdapter) NewTools(ctx context.Context) ([]mcp.Tool, error) { return c.ListTools(ctx) }

// MCPManager owns the MCP clients of one run and namespaces their tools.
type MCPManager struct {
	clients      map[string]mcpClient
	tools        map[string]mcpToolRef
	descriptions map[string]string
	specs        []map[string]any
	// Observe is called after each tool call (run events/metrics).
	Observe func(server, tool string, duration time.Duration, err error)
}

type mcpToolRef struct {
	server string
	tool   string
}

var mcpUnsafe = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// MCPToolName builds the namespaced tool name; built-in tools can never be
// shadowed because they do not use the mcp__ prefix.
func MCPToolName(server, tool string) string {
	return "mcp__" + mcpUnsafe.ReplaceAllString(server, "_") + "__" + mcpUnsafe.ReplaceAllString(tool, "_")
}

// StartMCP starts and initializes each server. A failed server is skipped and
// returned in the error list so the caller can emit an mcp.unavailable event.
func StartMCP(ctx context.Context, servers map[string]mcpClient) (*MCPManager, map[string]error) {
	manager := &MCPManager{clients: map[string]mcpClient{}, tools: map[string]mcpToolRef{}, descriptions: map[string]string{}}
	failures := map[string]error{}
	for name, client := range servers {
		if err := client.Initialize(ctx); err != nil {
			failures[name] = err
			_ = client.Close()
			continue
		}
		list, err := client.NewTools(ctx)
		if err != nil {
			failures[name] = err
			_ = client.Close()
			continue
		}
		manager.clients[name] = client
		for _, tool := range list {
			full := MCPToolName(name, tool.Name)
			manager.tools[full] = mcpToolRef{server: name, tool: tool.Name}
			manager.descriptions[full] = tool.Description
			manager.specs = append(manager.specs, toolSpec(full, tool))
		}
	}
	return manager, failures
}

func toolSpec(full string, tool mcp.Tool) map[string]any {
	schema := tool.InputSchema
	if len(schema) == 0 {
		schema = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	description := tool.Description
	if description == "" {
		description = "MCP tool " + tool.Name
	}
	return map[string]any{"type": "function", "function": map[string]any{
		"name": full, "description": description, "parameters": schema,
	}}
}

// Specs returns OpenAI-style tool specs for all healthy servers.
func (m *MCPManager) Specs() []map[string]any {
	if m == nil {
		return nil
	}
	return m.specs
}

// Call dispatches a namespaced MCP tool.
func (m *MCPManager) Call(ctx context.Context, name string, args json.RawMessage) (string, error) {
	if m == nil {
		return "", fmt.Errorf("mcp: no manager")
	}
	ref, ok := m.tools[name]
	if !ok {
		return "", fmt.Errorf("mcp: unknown tool %q", name)
	}
	started := time.Now()
	out, err := m.clients[ref.server].CallTool(ctx, ref.tool, args)
	if err == nil {
		diffLike := "+ " + strings.ReplaceAll(out, "\n", "\n+ ")
		if findings := secretscan.Scan(diffLike); len(findings) > 0 {
			out = "blocked: MCP tool output contains a possible credential"
			err = fmt.Errorf("mcp: %s/%s output blocked by the secret scanner", ref.server, ref.tool)
		}
	}
	if m.Observe != nil {
		m.Observe(ref.server, ref.tool, time.Since(started), err)
	}
	return out, err
}

// Close closes all clients.
func (m *MCPManager) Close() {
	if m == nil {
		return
	}
	for _, client := range m.clients {
		_ = client.Close()
	}
}

// NewMCPAdapter wraps a concrete client for Manager use.
func NewMCPAdapter(client *mcp.Client) mcpClient { return clientAdapter{client} }

// appendMCPTools adds namespaced MCP specs to the built-in tool specs.
func appendMCPTools(specs []map[string]any, manager *MCPManager) []map[string]any {
	if manager == nil {
		return specs
	}
	return append(specs, manager.Specs()...)
}

// ToolInfo is one discovered MCP tool.
type ToolInfo struct {
	Server      string `json:"server"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Tools lists discovered tools for the API/UI.
func (m *MCPManager) Tools() []ToolInfo {
	if m == nil {
		return nil
	}
	out := make([]ToolInfo, 0, len(m.tools))
	for full, ref := range m.tools {
		out = append(out, ToolInfo{Server: ref.server, Name: ref.tool, Description: m.descriptions[full]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Server == out[j].Server {
			return out[i].Name < out[j].Name
		}
		return out[i].Server < out[j].Server
	})
	return out
}

// Restrict keeps only tools matching the allowlist. Patterns are exact names or
// prefixes ending in "*" (e.g. mcp__fs__*). An empty list means no restriction.
func (m *MCPManager) Restrict(patterns []string) {
	if m == nil || len(patterns) == 0 {
		return
	}
	allowed := func(name string) bool {
		for _, pattern := range patterns {
			if strings.HasSuffix(pattern, "*") {
				if strings.HasPrefix(name, strings.TrimSuffix(pattern, "*")) {
					return true
				}
				continue
			}
			if name == pattern {
				return true
			}
		}
		return false
	}
	tools := map[string]mcpToolRef{}
	descriptions := map[string]string{}
	specs := make([]map[string]any, 0, len(m.specs))
	for _, spec := range m.specs {
		fn, _ := spec["function"].(map[string]any)
		name, _ := fn["name"].(string)
		if !allowed(name) {
			continue
		}
		tools[name] = m.tools[name]
		descriptions[name] = m.descriptions[name]
		specs = append(specs, spec)
	}
	m.tools, m.descriptions, m.specs = tools, descriptions, specs
}
