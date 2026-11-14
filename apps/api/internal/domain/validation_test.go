package domain

import (
	"strings"
	"testing"
)

func TestNewTaskValidation(t *testing.T) {
	tests := []struct {
		name      string
		orgID     string
		projectID string
		ref       string
		title     string
	}{
		{"missing org", "", "p", "T-1", "Title"},
		{"missing project", "o", "", "T-1", "Title"},
		{"missing ref", "o", "p", "", "Title"},
		{"missing title", "o", "p", "T-1", " "},
		{"title too long", "o", "p", "T-1", strings.Repeat("x", 201)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewTask(tc.orgID, tc.projectID, tc.ref, tc.title)
			assertErr(t, err, ErrValidation)
		})
	}
}

func TestTaskValidateStatusAndPriority(t *testing.T) {
	task := newReadyTask(t)
	task.Status = "bogus"
	assertErr(t, task.Validate(), ErrValidation)

	task = newReadyTask(t)
	task.Priority = "urgent"
	assertErr(t, task.Validate(), ErrValidation)

	task = newReadyTask(t)
	task.Acceptance = make([]AcceptanceCriterion, 51)
	assertErr(t, task.Validate(), ErrValidation)
}

func TestParseTaskStatus(t *testing.T) {
	if s, err := ParseTaskStatus(" in_progress "); err != nil || s != TaskInProgress {
		t.Fatalf("ParseTaskStatus = %q, %v", s, err)
	}
	if _, err := ParseTaskStatus("nope"); err == nil {
		t.Fatal("ParseTaskStatus accepted an unknown status")
	}
}

func TestPriorityValid(t *testing.T) {
	for _, p := range []Priority{PriorityLow, PriorityMed, PriorityHigh} {
		if !p.Valid() {
			t.Errorf("%q should be valid", p)
		}
	}
	if Priority("urgent").Valid() {
		t.Error("urgent should not be valid")
	}
}

func TestProjectStatusValidation(t *testing.T) {
	p, err := NewProject("org-1", "CRM", "crm")
	if err != nil {
		t.Fatal(err)
	}
	p.Status = "deleted"
	assertErr(t, p.Validate(), ErrValidation)

	p.Status = ProjectArchived
	if err := p.Validate(); err != nil {
		t.Fatalf("archived project should be valid: %v", err)
	}

	p.Name = strings.Repeat("x", 121)
	assertErr(t, p.Validate(), ErrValidation)
}

func TestOrgNameTooLong(t *testing.T) {
	o, err := NewOrg("Acme", "acme")
	if err != nil {
		t.Fatal(err)
	}
	o.Name = strings.Repeat("x", 81)
	assertErr(t, o.Validate(), ErrValidation)
}

func TestUserNameValidation(t *testing.T) {
	u, err := NewUser("a@acme.io", "Ana")
	if err != nil {
		t.Fatal(err)
	}
	u.Name = " "
	assertErr(t, u.Validate(), ErrValidation)

	u.Name = strings.Repeat("x", 81)
	assertErr(t, u.Validate(), ErrValidation)
}

func TestDoRLongTitle(t *testing.T) {
	task := newReadyTask(t)
	task.Title = strings.Repeat("x", 201)
	if missing := task.DoR(); len(missing) == 0 {
		t.Fatal("DoR() should report the long title")
	}
}

func TestTransitionUnknownStatus(t *testing.T) {
	task := newReadyTask(t)
	assertErr(t, task.Transition(TaskStatus("bogus"), TransitionOptions{Actor: ActorHuman}), ErrValidation)
}

func TestTransitionToSameStatus(t *testing.T) {
	task := inStatus(t, TaskReady)
	assertErr(t, task.Transition(TaskReady, TransitionOptions{Actor: ActorHuman}), ErrInvalidTransition)
}

func TestCanTransitionFromBlocked(t *testing.T) {
	task := inStatus(t, TaskBlocked)
	if !task.CanTransition(TaskReady) || !task.CanTransition(TaskCanceled) {
		t.Error("blocked should allow ready and canceled")
	}
	if task.CanTransition(TaskInProgress) {
		t.Error("blocked must not allow in_progress")
	}
}
