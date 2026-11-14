// Package agent runs a coding agent inside a sandbox and turns its output into
// platform events. The backend is replaceable: the platform owns the lifecycle,
// the audit trail and the guardrails, not the model loop.
package agent

import (
	"context"
	"time"
)

// EventType classifies what the backend produced.
type EventType string

const (
	EventText EventType = "assistant.text"
	EventDone EventType = "run.done"
)

// Event is one item in the run stream.
type Event struct {
	Type EventType
	Text string
}

// Spec describes one agent invocation.
type Spec struct {
	RunID            string
	Prompt           string
	Model            string
	Image            string
	Timeout          time.Duration
	TestCommand      string
	CaptureWorkspace bool
	// Workspace is a tar of an existing repository to extract before the agent
	// runs. Nil means an empty workspace.
	Workspace []byte
	// Budget (zero fields = no limit) and the prices used to cost a run.
	BudgetTokensIn   int
	BudgetTokensOut  int
	BudgetCostMicros int64
	BudgetWallClock  time.Duration
	PriceInPer1K     float64
	PriceOutPer1K    float64
	// ReadOnly removes write_file (planning and review agents).
	ReadOnly bool
	// MCPConfig overrides the backend's AIP_MCP_SERVERS for this run (project).
	MCPConfig string
	// AllowedTools restricts MCP tools (empty = all).
	AllowedTools []string
}

// Result is what a backend produced.
type Result struct {
	Diff         string
	TestExitCode int
	TestOutput   string
	Workspace    []byte
	InputTokens  int
	OutputTokens int
	Provider     string
	Model        string
}

// Backend executes an agent and streams its output.
type Backend interface {
	Name() string
	Run(ctx context.Context, spec Spec, emit func(Event) error) (Result, error)
}
