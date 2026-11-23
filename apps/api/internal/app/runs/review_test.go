package runs

import (
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

func TestReviewPromptIncludesTheDevelopmentDiff(t *testing.T) {
	t.Parallel()
	prompt := reviewPrompt("diff --git a/main.go b/main.go\n+package main\n")
	if !strings.Contains(prompt, "diff --git a/main.go b/main.go") {
		t.Fatalf("prompt does not contain the diff: %s", prompt)
	}
	if !strings.Contains(prompt, "hand-written cryptography") {
		t.Fatal("prompt must ask the reviewer to flag hand-written cryptography")
	}
	if !strings.Contains(prompt, "THESE changes") {
		t.Fatal("prompt must tell the agent to review the diff, not the base repository")
	}
	if !strings.Contains(prompt, "AIP_REVIEW") && !strings.Contains(prompt, "REVIEW agent") {
		t.Fatal("prompt must keep the review role")
	}
}

func TestReviewPromptWithoutDiffAsksForABlocker(t *testing.T) {
	t.Parallel()
	prompt := reviewPrompt("   ")
	if !strings.Contains(prompt, "blocker") || !strings.Contains(prompt, "nothing was delivered") {
		t.Fatalf("an empty diff must ask for a blocker finding, got: %s", prompt)
	}
	if strings.Contains(prompt, "```diff") {
		t.Fatal("an empty diff must not open a diff block")
	}
}

func TestReviewPromptTruncatesHugeDiffs(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("x", reviewDiffLimit+1000)
	prompt := reviewPrompt(huge)
	if !strings.Contains(prompt, "[diff truncated]") {
		t.Fatal("large diffs must be marked as truncated")
	}
	if len(prompt) > reviewDiffLimit+500 {
		t.Fatalf("prompt length %d exceeds the cap", len(prompt))
	}
}

func TestReviewPromptListsChangedFiles(t *testing.T) {
	t.Parallel()
	diff := "diff --git a/apps/api/internal/auth/password.go b/apps/api/internal/auth/password.go\n+++ b/apps/api/internal/auth/password.go\n+x\n" +
		"diff --git a/apps/api/internal/httpapi/changes.go b/apps/api/internal/httpapi/changes.go\n+++ b/apps/api/internal/httpapi/changes.go\n+y\n"
	prompt := reviewPrompt(diff)
	if !strings.Contains(prompt, "Changed files (2):") ||
		!strings.Contains(prompt, "- apps/api/internal/auth/password.go") ||
		!strings.Contains(prompt, "- apps/api/internal/httpapi/changes.go") {
		t.Fatalf("prompt does not index the changed files: %s", prompt)
	}
}

func TestParseReviewFindingsAcceptsBothFields(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"findings":[
		{"severity":"blocker","file":"a.go","line":3,"message":"the message"},
		{"severity":"nit","file":"b.go","body":"the body"},
		{"severity":"weird","body":"an unknown severity is a nit"}
	]}`)
	got := parseReviewFindings(raw)
	if len(got) != 3 {
		t.Fatalf("parseReviewFindings = %d findings, want 3", len(got))
	}
	if got[0].Message != "the message" || got[0].Severity != domain.ReviewFindingBlocker {
		t.Errorf("finding 0 = %+v, want the message and a blocker", got[0])
	}
	if got[1].Message != "the body" || got[1].Severity != domain.ReviewFindingNit {
		t.Errorf("finding 1 = %+v, want the body and a nit", got[1])
	}
	if got[2].Severity != domain.ReviewFindingNit {
		t.Errorf("finding 2 severity = %q, want nit: only blocker gates the merge", got[2].Severity)
	}
}

func TestParseReviewFindingsMajorGates(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"findings":[
		{"severity":"major","body":"the tests were weakened"},
		{"severity":"minor","body":"a style nit"},
		{"severity":"Blocker","body":"a real blocker"}
	]}`)
	got := parseReviewFindings(raw)
	if len(got) != 3 {
		t.Fatalf("findings = %d, want 3", len(got))
	}
	if got[0].Severity != domain.ReviewFindingBlocker {
		t.Errorf("a major must gate the merge, got %q", got[0].Severity)
	}
	if got[1].Severity != domain.ReviewFindingNit {
		t.Errorf("a minor stays advisory, got %q", got[1].Severity)
	}
	if got[2].Severity != domain.ReviewFindingBlocker {
		t.Errorf("a case-insensitive blocker must gate, got %q", got[2].Severity)
	}
}
