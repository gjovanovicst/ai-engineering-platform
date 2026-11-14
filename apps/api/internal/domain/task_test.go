package domain

import (
	"testing"
	"time"
)

func newReadyTask(t *testing.T) *Task {
	t.Helper()
	task, err := NewTask("org-1", "project-1", "T-142", "Add SSO login")
	if err != nil {
		t.Fatal(err)
	}
	task.Acceptance = []AcceptanceCriterion{{
		Given: "a user on /login",
		When:  "they click Sign in with SSO",
		Then:  "they are redirected to the IdP",
	}}
	return task
}

func TestTaskDoR(t *testing.T) {
	task := newReadyTask(t)
	if got := task.DoR(); len(got) != 0 {
		t.Fatalf("DoR() = %v, want empty", got)
	}

	noAC, _ := NewTask("org-1", "project-1", "T-1", "Do something")
	if missing := noAC.DoR(); len(missing) != 1 {
		t.Fatalf("DoR() = %v, want one missing criterion", missing)
	}

	incomplete := newReadyTask(t)
	incomplete.Acceptance = []AcceptanceCriterion{{Given: "x", When: "", Then: "z"}}
	if missing := incomplete.DoR(); len(missing) != 1 {
		t.Fatalf("DoR() = %v, want incomplete criterion", missing)
	}
}

func TestTaskHappyPath(t *testing.T) {
	task := newReadyTask(t)
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

	steps := []struct {
		to  TaskStatus
		opt TransitionOptions
	}{
		{TaskReady, TransitionOptions{Actor: ActorHuman, At: at}},
		{TaskPlanning, TransitionOptions{Actor: ActorAgent, At: at}},
		{TaskInProgress, TransitionOptions{Actor: ActorAgent, PlanApproved: true, At: at}},
		{TaskReview, TransitionOptions{Actor: ActorAgent, TestsGreen: true, At: at}},
		{TaskDone, TransitionOptions{Actor: ActorHuman, PRMerged: true, At: at}},
	}
	for _, step := range steps {
		if err := task.Transition(step.to, step.opt); err != nil {
			t.Fatalf("transition to %s: %v", step.to, err)
		}
	}
	if task.Status != TaskDone {
		t.Fatalf("status = %s, want done", task.Status)
	}
	if task.ReadyAt == nil || task.DoneAt == nil {
		t.Fatal("ReadyAt / DoneAt were not recorded")
	}
}

func TestTaskTransitionGuards(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T) *Task
		to      TaskStatus
		opt     TransitionOptions
		wantErr error
	}{
		{
			name: "ready requires complete AC",
			prepare: func(t *testing.T) *Task {
				return &Task{OrgID: "o", ProjectID: "p", Ref: "T-1", Title: "x", Status: TaskDraft, Priority: PriorityMed}
			},
			to:  TaskReady,
			opt: TransitionOptions{Actor: ActorHuman},
		},
		{
			name:    "planning to in_progress requires approved plan",
			prepare: func(t *testing.T) *Task { return inStatus(t, TaskPlanning) },
			to:      TaskInProgress,
			opt:     TransitionOptions{Actor: ActorAgent},
		},
		{
			name:    "review requires green tests",
			prepare: func(t *testing.T) *Task { return inStatus(t, TaskInProgress) },
			to:      TaskReview,
			opt:     TransitionOptions{Actor: ActorAgent},
		},
		{
			name:    "done requires merged PR",
			prepare: func(t *testing.T) *Task { return inStatus(t, TaskReview) },
			to:      TaskDone,
			opt:     TransitionOptions{Actor: ActorHuman},
		},
		{
			name:    "blocking requires a reason",
			prepare: func(t *testing.T) *Task { return inStatus(t, TaskReady) },
			to:      TaskBlocked,
			opt:     TransitionOptions{Actor: ActorHuman},
			wantErr: ErrValidation,
		},
		{
			name:    "agent cannot cancel",
			prepare: func(t *testing.T) *Task { return inStatus(t, TaskReady) },
			to:      TaskCanceled,
			opt:     TransitionOptions{Actor: ActorAgent},
		},
		{
			name:    "draft cannot jump to done",
			prepare: func(t *testing.T) *Task { return newReadyTask(t) },
			to:      TaskDone,
			opt:     TransitionOptions{Actor: ActorHuman, PRMerged: true},
		},
		{
			name:    "review cannot go back to planning",
			prepare: func(t *testing.T) *Task { return inStatus(t, TaskReview) },
			to:      TaskPlanning,
			opt:     TransitionOptions{Actor: ActorHuman},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := tc.prepare(t)
			err := task.Transition(tc.to, tc.opt)
			want := tc.wantErr
			if want == nil {
				want = ErrInvalidTransition
			}
			assertErr(t, err, want)
		})
	}
}

func TestBlockedRoundTrip(t *testing.T) {
	task := inStatus(t, TaskReady)
	if err := task.Transition(TaskBlocked, TransitionOptions{Actor: ActorHuman, Reason: "waiting for IdP access"}); err != nil {
		t.Fatal(err)
	}
	if task.BlockedReason != "waiting for IdP access" {
		t.Fatalf("BlockedReason = %q", task.BlockedReason)
	}
	if err := task.Transition(TaskReady, TransitionOptions{Actor: ActorHuman}); err != nil {
		t.Fatal(err)
	}
	if task.BlockedReason != "" {
		t.Fatalf("BlockedReason = %q, want cleared", task.BlockedReason)
	}
}

func TestTerminalStates(t *testing.T) {
	for _, status := range []TaskStatus{TaskDone, TaskCanceled} {
		task := inStatus(t, status)
		err := task.Transition(TaskReady, TransitionOptions{Actor: ActorHuman})
		assertErr(t, err, ErrInvalidTransition)
	}
}

func TestCanTransition(t *testing.T) {
	task := inStatus(t, TaskReview)
	if !task.CanTransition(TaskDone) {
		t.Error("review should allow done")
	}
	if task.CanTransition(TaskPlanning) {
		t.Error("review must not allow planning")
	}
}

func inStatus(t *testing.T, status TaskStatus) *Task {
	t.Helper()
	task := newReadyTask(t)
	task.Status = status
	return task
}
