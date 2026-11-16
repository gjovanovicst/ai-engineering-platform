package runs

import (
	"strings"
	"testing"
)

func TestReviewPromptIncludesTheDevelopmentDiff(t *testing.T) {
	t.Parallel()
	prompt := reviewPrompt("diff --git a/main.go b/main.go\n+package main\n")
	if !strings.Contains(prompt, "diff --git a/main.go b/main.go") {
		t.Fatalf("prompt does not contain the diff: %s", prompt)
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
