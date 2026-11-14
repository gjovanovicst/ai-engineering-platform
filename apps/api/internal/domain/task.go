package domain

import (
	"strconv"
	"strings"
	"time"
)

// TaskStatus is the position of a task in the ASDLC state machine
// (docs/01-asdlc.md §2). The transitions below are the only legal moves.
type TaskStatus string

const (
	TaskDraft      TaskStatus = "draft"
	TaskReady      TaskStatus = "ready"
	TaskPlanning   TaskStatus = "planning"
	TaskInProgress TaskStatus = "in_progress"
	TaskReview     TaskStatus = "review"
	TaskDone       TaskStatus = "done"
	TaskBlocked    TaskStatus = "blocked"
	TaskCanceled   TaskStatus = "canceled"
)

// Valid reports whether s is a known status.
func (s TaskStatus) Valid() bool {
	switch s {
	case TaskDraft, TaskReady, TaskPlanning, TaskInProgress, TaskReview, TaskDone, TaskBlocked, TaskCanceled:
		return true
	}
	return false
}

// Terminal reports whether the task can no longer move.
func (s TaskStatus) Terminal() bool {
	return s == TaskDone || s == TaskCanceled
}

// ParseTaskStatus converts an API string into a status.
func ParseTaskStatus(raw string) (TaskStatus, error) {
	s := TaskStatus(strings.TrimSpace(raw))
	if !s.Valid() {
		return "", invalid("status", "unknown task status")
	}
	return s, nil
}

// Priority is the task priority shown in the backlog.
type Priority string

const (
	PriorityLow  Priority = "low"
	PriorityMed  Priority = "med"
	PriorityHigh Priority = "high"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	return p == PriorityLow || p == PriorityMed || p == PriorityHigh
}

// AcceptanceCriterion is one Given/When/Then clause. A task without at least
// one complete criterion does not satisfy the Definition of Ready.
type AcceptanceCriterion struct {
	Given string
	When  string
	Then  string
}

// Complete reports whether every part of the criterion is present.
func (a AcceptanceCriterion) Complete() bool {
	return strings.TrimSpace(a.Given) != "" &&
		strings.TrimSpace(a.When) != "" &&
		strings.TrimSpace(a.Then) != ""
}

// Actor is who performs a transition. Agents may not cancel tasks and never
// move a task to done on their own.
type Actor string

const (
	ActorHuman  Actor = "human"
	ActorAgent  Actor = "agent"
	ActorSystem Actor = "system"
)

// Task is a unit of work in a project's backlog.
type Task struct {
	ID            string
	OrgID         string
	ProjectID     string
	Ref           string
	Title         string
	Description   string
	Acceptance    []AcceptanceCriterion
	Status        TaskStatus
	Priority      Priority
	Tags          []string
	AssigneeID    string
	CreatedBy     string
	BlockedReason string
	DependsOn     []string

	ReadyAt *time.Time
	DoneAt  *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTask validates and normalizes a new task in draft status.
func NewTask(orgID, projectID, ref, title string) (*Task, error) {
	t := &Task{
		OrgID:     strings.TrimSpace(orgID),
		ProjectID: strings.TrimSpace(projectID),
		Ref:       strings.TrimSpace(ref),
		Title:     strings.TrimSpace(title),
		Status:    TaskDraft,
		Priority:  PriorityMed,
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t, nil
}

// Validate enforces the invariants that hold in every status.
func (t *Task) Validate() error {
	if t.OrgID == "" {
		return invalid("org_id", "must not be empty")
	}
	if t.ProjectID == "" {
		return invalid("project_id", "must not be empty")
	}
	if t.Ref == "" {
		return invalid("ref", "must not be empty")
	}
	if strings.TrimSpace(t.Title) == "" {
		return invalid("title", "must not be empty")
	}
	if len(t.Title) > 200 {
		return invalid("title", "must be at most 200 characters")
	}
	if !t.Status.Valid() {
		return invalid("status", "unknown task status")
	}
	if !t.Priority.Valid() {
		return invalid("priority", "must be low, med or high")
	}
	if len(t.Acceptance) > 50 {
		return invalid("acceptance", "must have at most 50 criteria")
	}
	return nil
}

// DoR returns the unmet Definition of Ready items (docs/01-asdlc.md §4).
// Only the parts that are decidable inside the aggregate live here; scope,
// dependencies and estimates are checked by the application layer.
func (t *Task) DoR() []string {
	var missing []string
	if strings.TrimSpace(t.Title) == "" {
		missing = append(missing, "title is empty")
	}
	if len(t.Title) > 200 {
		missing = append(missing, "title is longer than 200 characters")
	}
	if len(t.Acceptance) == 0 {
		missing = append(missing, "at least one acceptance criterion is required")
	}
	for i, ac := range t.Acceptance {
		if !ac.Complete() {
			missing = append(missing, "acceptance criterion "+strconv.Itoa(i+1)+" is incomplete (given/when/then)")
		}
	}
	return missing
}

// ReadyForWork reports whether the task satisfies the domain-level DoR.
func (t *Task) ReadyForWork() bool { return len(t.DoR()) == 0 }

// TransitionOptions carries the facts the state machine needs. The domain does
// not look anything up; the caller passes what it already checked.
type TransitionOptions struct {
	Actor        Actor
	Reason       string
	PlanApproved bool
	TestsGreen   bool
	PRMerged     bool
	At           time.Time
}

// TransitionError describes a rejected move and lists what would have been legal.
type TransitionError struct {
	From   TaskStatus
	To     TaskStatus
	Detail string
}

func (e *TransitionError) Error() string {
	msg := "cannot move task from " + string(e.From) + " to " + string(e.To)
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	return msg
}

// Is lets callers match with errors.Is(err, ErrInvalidTransition).
func (e *TransitionError) Is(target error) bool { return target == ErrInvalidTransition }

var allowedTransitions = map[TaskStatus][]TaskStatus{
	TaskDraft:      {TaskReady, TaskBlocked, TaskCanceled},
	TaskReady:      {TaskPlanning, TaskBlocked, TaskCanceled},
	TaskPlanning:   {TaskInProgress, TaskBlocked, TaskCanceled},
	TaskInProgress: {TaskReview, TaskBlocked, TaskCanceled},
	TaskReview:     {TaskDone, TaskBlocked, TaskCanceled},
	TaskBlocked:    {TaskReady, TaskCanceled},
	TaskDone:       {},
	TaskCanceled:   {},
}

// CanTransition reports whether the move is structurally allowed, without
// checking the facts (plan approved, tests green, ...).
func (t *Task) CanTransition(to TaskStatus) bool {
	for _, allowed := range allowedTransitions[t.Status] {
		if allowed == to {
			return true
		}
	}
	return false
}

// Transition applies the state machine. It is the only way to change Status.
func (t *Task) Transition(to TaskStatus, opt TransitionOptions) error {
	if !to.Valid() {
		return invalid("to", "unknown task status")
	}
	if t.Status.Terminal() {
		return &TransitionError{From: t.Status, To: to, Detail: "the task is in a terminal state"}
	}
	if to == t.Status {
		return &TransitionError{From: t.Status, To: to, Detail: "already in that state"}
	}
	if !t.CanTransition(to) {
		return &TransitionError{From: t.Status, To: to, Detail: "not an allowed transition"}
	}

	if to == TaskBlocked && strings.TrimSpace(opt.Reason) == "" {
		return invalid("reason", "blocking a task requires a reason")
	}
	if to == TaskCanceled && opt.Actor != ActorHuman {
		return &TransitionError{From: t.Status, To: to, Detail: "only a human can cancel a task"}
	}
	if to == TaskReady && t.Status == TaskDraft && !t.ReadyForWork() {
		return &TransitionError{
			From:   t.Status,
			To:     to,
			Detail: "definition of ready is not met: " + strings.Join(t.DoR(), "; "),
		}
	}
	if to == TaskInProgress && !opt.PlanApproved {
		return &TransitionError{From: t.Status, To: to, Detail: "the plan has not been approved"}
	}
	if to == TaskReview && !opt.TestsGreen {
		return &TransitionError{From: t.Status, To: to, Detail: "the test gate is not green"}
	}
	if to == TaskDone && !opt.PRMerged {
		return &TransitionError{From: t.Status, To: to, Detail: "the pull request has not been merged"}
	}

	at := opt.At
	if at.IsZero() {
		at = time.Now().UTC()
	}

	t.Status = to
	t.UpdatedAt = at
	t.BlockedReason = ""
	if to == TaskBlocked {
		t.BlockedReason = strings.TrimSpace(opt.Reason)
	}
	if to == TaskReady && t.ReadyAt == nil {
		t.ReadyAt = &at
	}
	if to == TaskDone {
		t.DoneAt = &at
	}
	return nil
}
