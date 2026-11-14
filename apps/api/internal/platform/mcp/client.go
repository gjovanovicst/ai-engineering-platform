// Package mcp implements the client side of the Model Context Protocol over
// stdio (newline-delimited JSON-RPC 2.0).
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Tool is one MCP tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Client is one running MCP server.
type Client struct {
	stdin   io.WriteCloser
	sc      *bufio.Scanner
	closeFn func() error
	mu      sync.Mutex
	next    int
}

type request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type response struct {
	ID     *int            `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Start launches a trusted MCP server command with the current environment.
func Start(ctx context.Context, command string, args ...string) (*Client, error) {
	return StartWithEnv(ctx, command, nil, args...)
}

// StartWithEnv is Start with an explicit environment (tests, secret allowlists).
func StartWithEnv(ctx context.Context, command string, env []string, args ...string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec // command is trusted operator configuration, never model output
	if env != nil {
		cmd.Env = env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return NewClient(stdin, stdout, func() error {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return cmd.Wait()
	}), nil
}

// NewClient adapts any newline JSON-RPC stream (sandbox process, pipe, test).
func NewClient(in io.WriteCloser, out io.Reader, closeFn func() error) *Client {
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	if closeFn == nil {
		closeFn = func() error { return nil }
	}
	return &Client{stdin: in, sc: sc, closeFn: closeFn}
}

// Initialize performs the MCP handshake.
func (c *Client) Initialize(ctx context.Context) error {
	_, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "aip", "version": "dev"},
	})
	if err != nil {
		return err
	}
	return c.notify(ctx, "notifications/initialized", map[string]any{})
}

// ListTools returns the tools the server exposes.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("mcp: decode tools: %w", err)
	}
	return payload.Tools, nil
}

// CallTool calls one tool and returns its text content.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (string, error) {
	raw, err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", err
	}
	var payload struct {
		IsError bool `json:"isError"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("mcp: decode tool result: %w", err)
	}
	out := ""
	for _, item := range payload.Content {
		if item.Type == "text" {
			out += item.Text
		}
	}
	if payload.IsError {
		return out, errors.New("mcp: tool returned an error")
	}
	return out, nil
}

// Close terminates the server.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeFn()
}

func (c *Client) notify(ctx context.Context, method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if _, err := c.stdin.Write(append(payload, '\n')); err != nil {
		return err
	}
	return nil
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	id := c.next
	payload, _ := json.Marshal(request{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if _, err := c.stdin.Write(append(payload, '\n')); err != nil {
		return nil, err
	}

	type result struct {
		raw json.RawMessage
		err error
	}
	ch := make(chan result, 1)
	go func() {
		for c.sc.Scan() {
			var msg response
			if err := json.Unmarshal(c.sc.Bytes(), &msg); err != nil {
				continue
			}
			if msg.ID == nil || *msg.ID != id {
				continue
			}
			if msg.Error != nil {
				ch <- result{err: fmt.Errorf("mcp: %s: %s", method, msg.Error.Message)}
				return
			}
			ch <- result{raw: msg.Result}
			return
		}
		if err := c.sc.Err(); err != nil {
			ch <- result{err: err}
			return
		}
		ch <- result{err: errors.New("mcp: server closed the stream")}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.raw, res.err
	}
}
