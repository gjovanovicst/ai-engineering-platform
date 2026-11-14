// Package aipclient is a small HTTP client for the platform API (CLI/IDE).
package aipclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to /api/v1 with a bearer token.
type Client struct {
	BaseURL string
	OrgID   string
	Token   string
	HTTP    *http.Client
}

// New validates the client configuration.
func New(baseURL, orgID, token string) (*Client, error) {
	if baseURL == "" || orgID == "" || token == "" {
		return nil, fmt.Errorf("AIP_API_URL, AIP_ORG_ID and AIP_API_TOKEN are required")
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), OrgID: orgID, Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		var problem struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(data, &problem)
		if problem.Detail == "" {
			problem.Detail = strings.TrimSpace(string(data))
		}
		return fmt.Errorf("api: status %d: %s", res.StatusCode, problem.Detail)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// Task is the CLI view of a task.
type Task struct {
	ID     string `json:"id"`
	Ref    string `json:"ref"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// ListTasks lists a project's tasks.
func (c *Client) ListTasks(ctx context.Context, projectID string) ([]Task, error) {
	var out struct {
		Data []Task `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "/api/v1/orgs/"+c.OrgID+"/projects/"+projectID+"/tasks", nil, &out)
	return out.Data, err
}

// Run is the CLI view of a run.
type Run struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	AgentType   string `json:"agent_type"`
	FailureCode string `json:"failure_code"`
}

// StartRun queues an agent run.
func (c *Client) StartRun(ctx context.Context, projectID, taskRef, agentType string) (Run, error) {
	var out Run
	err := c.do(ctx, http.MethodPost, "/api/v1/orgs/"+c.OrgID+"/projects/"+projectID+"/tasks/"+taskRef+"/runs",
		map[string]string{"agent_type": agentType}, &out)
	return out, err
}

// GetRun fetches one run.
func (c *Client) GetRun(ctx context.Context, runID string) (Run, error) {
	var out Run
	err := c.do(ctx, http.MethodGet, "/api/v1/orgs/"+c.OrgID+"/runs/"+runID, nil, &out)
	return out, err
}

// Decide approves or rejects an approval.
func (c *Client) Decide(ctx context.Context, approvalID, decision, reason string) error {
	return c.do(ctx, http.MethodPost, "/api/v1/orgs/"+c.OrgID+"/approvals/"+approvalID+"/decision",
		map[string]string{"decision": decision, "reason": reason}, nil)
}
