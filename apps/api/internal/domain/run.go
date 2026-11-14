package domain

import (
	"strings"
	"time"
)

// RunStatus is the lifecycle of one agent execution (docs/01-asdlc.md §3).
type RunStatus string

const (
	RunQueued           RunStatus = "queued"
	RunRunning          RunStatus = "running"
	RunAwaitingApproval RunStatus = "awaiting_approval"
	RunSucceeded        RunStatus = "succeeded"
	RunFailed           RunStatus = "failed"
	RunCanceled         RunStatus = "canceled"
)

// Valid reports whether s is a known run status.
func (s RunStatus) Valid() bool {
	switch s {
	case RunQueued, RunRunning, RunAwaitingApproval, RunSucceeded, RunFailed, RunCanceled:
		return true
	}
	return false
}

// Terminal reports whether the run can no longer move.
func (s RunStatus) Terminal() bool {
	return s == RunSucceeded || s == RunFailed || s == RunCanceled
}

// PhaseName is a step inside a run.
type PhaseName string

const (
	PhaseContext   PhaseName = "context"
	PhasePlan      PhaseName = "plan"
	PhaseImplement PhaseName = "implement"
	PhaseTest      PhaseName = "test"
	PhaseReview    PhaseName = "review"
	PhaseDiff      PhaseName = "diff"
	PhasePR        PhaseName = "pr"
)

// Run is one execution of an agent over a task.
// Budget limits a single run. Zero fields mean "no limit".
type Budget struct {
	TokensIn   int   `json:"tokens_in,omitempty"`
	TokensOut  int   `json:"tokens_out,omitempty"`
	CostMicros int64 `json:"cost_micros,omitempty"`
	WallClockS int   `json:"wall_clock_s,omitempty"`
}

type Run struct {
	ID           string
	OrgID        string
	ProjectID    string
	TaskID       string
	AgentType    string
	Status       RunStatus
	CurrentPhase PhaseName
	Model        string
	Budget       Budget
	Plan         []byte
	Diff         []byte
	Review       []byte
	// AcceptanceProof is populated in memory after proof validation so the PR
	// body can render it; the proof artifact is the persisted copy.
	AcceptanceProof      []byte
	ContextHash          string
	FrozenContextHash    string
	FrozenContextSources string
	TraceID              string
	FailureCode          string
	FailureDetail        string
	CancelRequestedAt    *time.Time
	StartedAt            *time.Time
	FinishedAt           *time.Time
	CreatedAt            time.Time
}

var runTransitions = map[RunStatus][]RunStatus{
	RunQueued:           {RunRunning, RunCanceled},
	RunRunning:          {RunAwaitingApproval, RunSucceeded, RunFailed, RunCanceled},
	RunAwaitingApproval: {RunRunning, RunFailed, RunCanceled},
	RunSucceeded:        {},
	RunFailed:           {},
	RunCanceled:         {},
}

// CanTransition reports whether the move is structurally allowed.
func (r *Run) CanTransition(to RunStatus) bool {
	for _, allowed := range runTransitions[r.Status] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Transition applies the run state machine.
func (r *Run) Transition(to RunStatus, at time.Time) error {
	if !to.Valid() {
		return &TransitionError{From: TaskStatus(r.Status), To: TaskStatus(to), Detail: "unknown run status"}
	}
	if r.Status == to {
		return &TransitionError{From: TaskStatus(r.Status), To: TaskStatus(to), Detail: "already in that state"}
	}
	if !r.CanTransition(to) {
		return &TransitionError{From: TaskStatus(r.Status), To: TaskStatus(to), Detail: "not an allowed transition"}
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	r.Status = to
	if to == RunRunning && r.StartedAt == nil {
		r.StartedAt = &at
	}
	if to.Terminal() {
		r.FinishedAt = &at
	}
	return nil
}

// AgentType is the kind of work a run performs.
type AgentType string

const (
	AgentPlanning      AgentType = "planning"
	AgentDevelopment   AgentType = "development"
	AgentTesting       AgentType = "testing"
	AgentReview        AgentType = "review"
	AgentDocumentation AgentType = "documentation"
)

// Valid reports whether a is a known agent type.
func (a AgentType) Valid() bool {
	switch a {
	case AgentPlanning, AgentDevelopment, AgentTesting, AgentReview, AgentDocumentation:
		return true
	}
	return false
}

// ParseAgentType converts an API string.
func ParseAgentType(raw string) (AgentType, error) {
	a := AgentType(strings.TrimSpace(raw))
	if !a.Valid() {
		return "", invalid("agent_type", "unknown agent type")
	}
	return a, nil
}
