package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent/sandbox"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/mcp"
)

// MCPServer is one trusted MCP server configuration.
type MCPServer struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	// AllowNetwork grants this server's own sandbox network access.
	AllowNetwork bool `json:"allow_network,omitempty"`
}

// ParseMCPServers parses AIP_MCP_SERVERS. Invalid or unsafe entries are errors;
// the caller decides whether to fail the run or log and continue.
func ParseMCPServers(raw string) ([]MCPServer, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var servers []MCPServer
	if err := json.Unmarshal([]byte(raw), &servers); err != nil {
		return nil, fmt.Errorf("AIP_MCP_SERVERS: %w", err)
	}
	seen := map[string]bool{}
	for i := range servers {
		servers[i].Name = strings.TrimSpace(servers[i].Name)
		servers[i].Command = strings.TrimSpace(servers[i].Command)
		if servers[i].Name == "" || servers[i].Command == "" {
			return nil, fmt.Errorf("AIP_MCP_SERVERS: every server needs name and command")
		}
		if seen[servers[i].Name] {
			return nil, fmt.Errorf("AIP_MCP_SERVERS: duplicate server %q", servers[i].Name)
		}
		seen[servers[i].Name] = true
	}
	return servers, nil
}

// StartMCPInSandbox starts each server inside the run sandbox and initializes
// the MCP handshake. Failed servers are returned so the caller can emit events.
func StartMCPInSandbox(ctx context.Context, sb sandbox.Sandbox, handle sandbox.Handle, servers []MCPServer) (*MCPManager, map[string]error) {
	clients := map[string]mcpClient{}
	for _, server := range servers {
		argv := append([]string{server.Command}, server.Args...)
		proc, err := sb.StartProcess(ctx, handle, argv, server.Env)
		if err != nil {
			// StartMCP reports failures per name; a client that cannot start is
			// represented by a failing stub.
			clients[server.Name] = failingMCPClient{err: err}
			continue
		}
		client := mcp.NewClient(proc.Stdin(), proc.Stdout(), func() error {
			_ = proc.Kill()
			return proc.Wait()
		})
		clients[server.Name] = NewMCPAdapter(client)
	}
	return StartMCP(ctx, clients)
}

type failingMCPClient struct{ err error }

func (f failingMCPClient) Initialize(context.Context) error { return f.err }
func (f failingMCPClient) NewTools(context.Context) ([]mcp.Tool, error) {
	return nil, f.err
}
func (f failingMCPClient) CallTool(context.Context, string, json.RawMessage) (string, error) {
	return "", f.err
}
func (f failingMCPClient) Close() error { return nil }

// mustMCPServers parses config; invalid config disables MCP rather than failing
// the run (the run still has its built-in tools).
func mustMCPServers(raw string) []MCPServer {
	servers, err := ParseMCPServers(raw)
	if err != nil {
		return nil
	}
	return servers
}

// effectiveMCPConfig prefers the per-run project config over the environment.
func effectiveMCPConfig(runConfig, envConfig string) string {
	if strings.TrimSpace(runConfig) != "" {
		return runConfig
	}
	return envConfig
}

// StartMCPIsolated gives every server its own sandbox (and therefore its own
// network policy); the run sandbox is not shared. Closing the manager destroys
// each server sandbox.
func StartMCPIsolated(ctx context.Context, sb sandbox.Sandbox, image string, servers []MCPServer) (*MCPManager, map[string]error) {
	clients := map[string]mcpClient{}
	for _, server := range servers {
		handle, err := sb.Create(ctx, sandbox.Spec{Image: image, Workdir: "/workspace", NetworkDisabled: !server.AllowNetwork, CPU: 1, MemoryMB: 512})
		if err != nil {
			clients[server.Name] = failingMCPClient{err: err}
			continue
		}
		argv := append([]string{server.Command}, server.Args...)
		proc, err := sb.StartProcess(ctx, handle, argv, server.Env)
		if err != nil {
			_ = sb.Destroy(ctx, handle)
			clients[server.Name] = failingMCPClient{err: err}
			continue
		}
		h := handle
		client := mcp.NewClient(proc.Stdin(), proc.Stdout(), func() error {
			_ = proc.Kill()
			err := proc.Wait()
			_ = sb.Destroy(context.WithoutCancel(ctx), h)
			return err
		})
		clients[server.Name] = NewMCPAdapter(client)
	}
	return StartMCP(ctx, clients)
}
